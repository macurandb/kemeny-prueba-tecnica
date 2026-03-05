# Arquitectura y Decisiones

## 1. Code Review — Resumen Ejecutivo

### Los 3 problemas mas graves

**1. JWT `exp` como string — Tokens nunca expiran (CRITICO)**

`handler/tasks.go:547` usa `fmt.Sprintf("%d", ...)` para generar el claim `exp`, produciendo un string `"1741209600"` en vez del numero `1741209600` que requiere el RFC 7519. La libreria `golang-jwt/v5` espera un `float64` para la validacion de expiracion; al encontrar un string, la asercion de tipo falla silenciosamente y **la validacion se omite por completo**. Todo token emitido es valido para siempre.

Lo priorice primero porque **anula toda la seguridad de autenticacion**. Un token robado (filtrado en logs, interceptado en red, o de un empleado que dejo la empresa) da acceso permanente al sistema. Es un fix de 5 minutos (quitar el `Sprintf`) con impacto maximo.

**2. Edit history registra old_value incorrecto (CRITICO)**

`handler/tasks.go:305` primero muta `existing.Status = *req.Status` y despues en linea 339 registra `existing.Status` como `old_value`. En ese punto ya contiene el valor nuevo, asi que `old_value == new_value`. Se pierde el dato original.

Lo priorice segundo porque **corrompe silenciosamente datos de auditoria**. Cualquier funcionalidad de trazabilidad, rollback o compliance queda comprometida. Es invisible hasta que alguien intenta usar el historial y descubre que todos los registros tienen el mismo valor en ambos campos. Fix de 10 minutos: capturar `oldStatus := existing.Status` antes de mutar.

**3. Error de UPDATE descartado silenciosamente (CRITICO)**

`handler/tasks.go:325` usa `_, _ = db.Pool.Exec(...)` descartando tanto el error como el `CommandTag`. Si el UPDATE falla (constraint violation, conexion perdida, deadlock), el handler continua, re-lee la fila sin cambios y la retorna como si la actualizacion hubiera sido exitosa.

Lo priorice tercero porque **el usuario pierde datos sin saberlo**. Cree que guardo su cambio, pero no fue asi. Esto destruye la confianza en la aplicacion y puede causar decisiones incorrectas basadas en datos que nunca se persistieron. Fix de 10 minutos: verificar el error y retornar 500.

### Criterio de priorizacion

Segui el criterio: **seguridad > integridad de datos > funcionalidad rota**. Los tres son fixes triviales (<15 min) con impacto desproporcionado. La relacion esfuerzo/valor es la mejor de los 28 problemas identificados en [REVIEW.md](./REVIEW.md).

---

## 2. Integracion de IA

### Modelo y provider

Elegi **langchaingo** (`github.com/tmc/langchaingo`) como libreria de integracion. Es el port de LangChain para Go — aunque su comunidad es mas pequena que la de Python, hereda el saber hacer y patrones probados de la comunidad Python (prompt templates, model abstraction, output parsing), sin perder las ventajas de performance de Go (binario compilado, bajo uso de memoria, concurrencia nativa).

La arquitectura soporta multiples providers via una interface `LLMClient`:

```
LLMClient (interface)
├── MockClient      → Clasificacion por keywords (sin API key, testing/dev)
├── LangChainClient → Wrapper sobre langchaingo
│   ├── OpenAI      → gpt-4o-mini (default), configurable via LLM_MODEL
│   └── Ollama      → llama3.2 (default), local sin API key
```

El provider se selecciona via `LLM_PROVIDER` env var, con **fallback resiliente**: si la inicializacion del provider falla (API key faltante, conexion rechazada), cae automaticamente a `MockClient` con un log de warning. Esto garantiza que la aplicacion siempre arranca, incluso sin configuracion de LLM.

Elegi `gpt-4o-mini` como modelo default para OpenAI por su balance entre costo (~$0.15/1M tokens input), velocidad (<1s), y calidad suficiente para clasificacion de texto corto. Para entornos sin acceso a APIs externas, Ollama con `llama3.2` permite correr clasificacion 100% local.

### Diseno del prompt

El prompt paso por algunas iteraciones:

**Iteracion 1 — Prompt libre**: Pedia al modelo que clasificara la tarea sin formato especifico. El problema fue que cada modelo retornaba formatos distintos (a veces YAML, a veces texto libre), haciendo el parsing impredecible.

**Iteracion 2 — JSON explicito con reglas**: Cambie a pedir respuesta JSON con formato exacto y reglas de validacion en el prompt:
```
Respond with this exact JSON format:
{
  "tags": ["tag1", "tag2"],
  "priority": "low|medium|high|urgent",
  "category": "bug|feature|improvement|research",
  "summary": "One-line summary of the task"
}

Rules:
- tags: Choose from: backend, frontend, bug, feature, devops, security, performance, documentation, testing. Max 4 tags.
- priority: Based on urgency and impact.
- category: Choose exactly one.
- summary: Max 80 characters.
```

Esto mejoro la consistencia, pero algunos modelos aun envolvian el JSON en bloques markdown (` ```json ... ``` `).

**Iteracion 3 (final) — Parsing robusto**: Mantuve el prompt de la iteracion 2 pero agregue:
- `extractJSON()` que detecta y elimina fences de markdown
- `validateClassification()` que corrige valores invalidos con defaults seguros (categoria invalida → `"feature"`, prioridad invalida → `"medium"`, tags desconocidos filtrados, summary truncado a 80 chars)
- `temperature=0.2` para respuestas deterministas
- `max_tokens=256` para control de costos

### Manejo de fallos del LLM

1. **Timeout**: Context con 30 segundos de deadline. Si el LLM no responde, retorno 500 con mensaje claro.
2. **Respuesta malformada**: `extractJSON` limpia markdown fences, `json.Unmarshal` valida JSON valido, `validateClassification` corrige valores fuera de rango.
3. **Provider caido**: El factory ya maneja esto con fallback a mock. Si el provider falla en runtime, el error se propaga al handler que retorna 500.
4. **Datos invalidos del modelo**: Nunca confio en la salida del LLM. Toda clasificacion pasa por validacion server-side con whitelist de valores validos.

### Costos en produccion

Para manejar costos con volumen real:

1. **Cache por contenido**: Hash de `SHA256(title + description)` como key en Redis. Si una tarea similar ya fue clasificada, retornar el resultado cacheado. TTL de 24h. Esto elimina re-clasificaciones costosas en tareas editadas minimamente.
2. **Rate limiting por usuario**: Maximo 10 clasificaciones por minuto por usuario via middleware. Previene abuso accidental (loops) o intencional.
3. **Modelo economico**: `gpt-4o-mini` a ~$0.15/1M tokens. Una clasificacion tipica usa ~200 tokens input + ~100 output = ~$0.00005/clasificacion. A 10K tareas/mes = ~$0.50/mes.
4. **Circuit breaker**: Si el LLM falla 5 veces consecutivas, switchear automaticamente a mock por 5 minutos antes de reintentar.

### Clasificar 10,000 tareas existentes

No procesaria las 10K de golpe. Mi approach:

1. **Cola de trabajo**: Encolar las 10K tareas en SQS/Redis queue con prioridad por fecha de creacion (mas recientes primero).
2. **Worker pool**: 5 workers concurrentes procesando la cola. Cada worker toma una tarea, la clasifica, persiste, y pasa a la siguiente.
3. **Rate limiting al API**: Respetar los rate limits del provider (OpenAI: ~500 RPM para gpt-4o-mini). Con 5 workers y respuesta promedio de 1s, serian ~300 RPM — dentro del limite.
4. **Idempotencia**: Verificar si la tarea ya tiene clasificacion AI antes de procesarla. `ON CONFLICT` en la persistencia previene duplicados.
5. **Progreso observable**: Contador en Redis con tareas procesadas/totales. Endpoint de status para monitorear progreso.
6. **Estimacion**: 10K tareas * ~1s/clasificacion / 5 workers = ~33 minutos. Costo: ~$0.50 con gpt-4o-mini.

---

## 3. Docker y Orquestacion

### Arquitectura

```
         Browser (localhost / app.kemeny.studio)
                      │
                   :80 (nginx)
                      │
                  ┌───┴───┐
                  │ Nginx │  reverse proxy
                  └───┬───┘
                      │
        ┌─────────────┼─────────────┐
        │ /*          │ /api/*      │
        │             │ /health     │
        ▼             ▼             │
    frontend       backend         │
    :3000          :8080           db
   (interno)     (interno)       :5432
                                (interno)
```

### Decisiones clave

**1. Nginx como reverse proxy (no exponer puertos directamente)**

Solo Nginx expone el puerto 80 al host. Frontend y backend son servicios internos de la red Docker, inaccesibles desde fuera. Esto tiene tres ventajas:
- **Seguridad**: Superficie de ataque reducida a un solo punto de entrada.
- **URLs relativas**: El frontend hace requests a `/api/tasks` (mismo host), eliminando la necesidad de `NEXT_PUBLIC_API_URL`. El mismo build funciona en dev (`localhost`) y prod (`app.kemeny.studio`).
- **Flexibilidad futura**: Puedo agregar SSL termination, rate limiting, o caching en Nginx sin tocar el codigo de la aplicacion.

**2. Multi-stage builds**

Ambos Dockerfiles usan multi-stage para minimizar el tamano de la imagen final:

- **Backend** (golang:1.24-alpine → alpine:3.21): La imagen final solo contiene el binario compilado (~15MB) y ca-certificates. No incluye Go compiler ni codigo fuente.
- **Frontend** (node:20-alpine → 3 stages): deps → builder → runner (standalone). La imagen final contiene solo el output de `next build --standalone` sin `node_modules` completo.

**3. Health check en PostgreSQL**

```yaml
healthcheck:
  test: ["CMD-SHELL", "pg_isready -U postgres"]
  interval: 2s
  timeout: 5s
  retries: 10
```

El backend usa `depends_on: db: condition: service_healthy`. Esto garantiza que la BD este lista para aceptar conexiones antes de que el backend intente conectarse. Sin esto, el backend arranca antes que PostgreSQL termine de inicializar (init.sql) y falla al conectar.

**4. Volume para persistencia de datos**

```yaml
volumes:
  - pgdata:/var/lib/postgresql/data
```

Named volume para que los datos sobrevivan a `docker-compose down` (sin `-v`). El seed data (`init.sql`) solo se ejecuta en la primera inicializacion gracias al mecanismo de `docker-entrypoint-initdb.d/`.

**5. CORS configurable via env var**

```yaml
backend:
  environment:
    CORS_ORIGIN: http://localhost
```

En vez de hardcodear `http://localhost:3000`, ahora el backend lee `CORS_ORIGIN` del environment. Esto permite:
- Dev sin Docker: default `http://localhost:3000`
- Docker local: `http://localhost` (a traves de Nginx en puerto 80)
- Produccion: `https://app.kemeny.studio`

**6. WebSocket headers en Nginx**

```nginx
location / {
    proxy_pass http://frontend;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

Headers de upgrade para que Next.js HMR (Hot Module Replacement) funcione a traves del proxy durante desarrollo. Sin esto, el dev server pierde la conexion WebSocket y no actualiza automaticamente.

---

## 4. Arquitectura para Produccion

Si este proyecto fuera a produccion con 10,000 usuarios concurrentes, esta es la arquitectura que implementaria:

### Diagrama

```
                        ┌─────────────┐
                        │ CloudFront  │ CDN (static assets, SSL)
                        └──────┬──────┘
                               │
                        ┌──────┴──────┐
                        │     ALB     │ Application Load Balancer
                        └──────┬──────┘
                               │
                 ┌─────────────┼─────────────┐
                 │             │             │
           ┌─────┴─────┐ ┌─────┴─────┐ ┌─────┴─────┐
           │ ECS Task  │ │ ECS Task  │ │ ECS Task  │  Backend (auto-scaling)
           │  backend  │ │  backend  │ │  backend  │
           └─────┬─────┘ └─────┬─────┘ └─────┬─────┘
                 │             │             │
        ┌────────┴─────────────┴─────────────┴────────┐
        │                                              │
  ┌─────┴─────┐  ┌──────────────┐  ┌─────────────────┐
  │   RDS     │  │ ElastiCache  │  │      SQS        │
  │ PostgreSQL│  │   (Redis)    │  │ (classify queue) │
  │ Multi-AZ  │  │   cluster    │  │                 │
  └───────────┘  └──────────────┘  └────────┬────────┘
                                            │
                                   ┌────────┴────────┐
                                   │  ECS Workers    │
                                   │ (classify tasks)│
                                   └─────────────────┘

  Frontend: Next.js pre-built → S3 + CloudFront (static export)
```

### Cambios en la arquitectura

**1. Frontend estatico en S3 + CloudFront**

Next.js con `output: 'export'` genera HTML/JS/CSS estatico servido desde S3 via CloudFront. Elimina la necesidad de un servidor Node.js en produccion. Latencia global baja, costo minimo, y cacheabilidad total.

**2. Backend horizontal con ECS Fargate**

El backend Go se despliega como servicio ECS Fargate con auto-scaling basado en CPU/memoria. Con el binario actual de ~15MB, cada tarea ECS usa recursos minimos. Auto-scaling: min 2 tasks (alta disponibilidad), max 10, target CPU 70%.

**3. RDS PostgreSQL Multi-AZ**

Instancia `db.r6g.large` con replica standby en otra AZ para failover automatico. Read replicas para queries de lectura pesadas (dashboard stats, listados). Connection pooling con PgBouncer como sidecar para manejar 10K conexiones concurrentes sin agotar el pool de PostgreSQL (default 100 conexiones).

**4. Redis (ElastiCache) para cache y sessions**

- Cache de clasificaciones LLM (key: hash de titulo+descripcion, TTL 24h)
- Rate limiting por usuario (sliding window)
- Cache de dashboard stats (TTL 30s — datos que no necesitan ser real-time)
- Session store si se migra de JWT a sessions

**5. SQS para clasificacion asincrona**

Las clasificaciones LLM se encolan en SQS en vez de ejecutarse sincrono en el request. El endpoint retorna 202 Accepted y el worker procesa en background. Esto evita que un LLM lento bloquee la respuesta HTTP y permite retry automatico si el provider falla.

**6. Graceful shutdown (prerequisito)**

Implementar shutdown controlado para zero-downtime deploys durante rolling updates en ECS. Sin esto, requests en vuelo se pierden en cada deploy.

### Deployment

- **IaC**: Terraform para toda la infraestructura AWS
- **CI/CD**: GitHub Actions → build Docker image → push a ECR → deploy a ECS
- **Stages**: `staging` (auto-deploy en merge a main) → `production` (manual approval)
- **Monitoring**: CloudWatch para metricas + alertas, X-Ray para tracing distribuido
- **Secrets**: AWS Secrets Manager para JWT_SECRET, DB_PASSWORD, API keys (nunca env vars en texto plano)

---

## 5. Trade-offs

### Nginx proxy vs NEXT_PUBLIC_API_URL

| | Nginx proxy | URL por ambiente |
|---|---|---|
| **Build** | Unico build para todos los ambientes | Rebuild por cada ambiente |
| **Complejidad** | Agrega un servicio mas (Nginx) | Solo frontend y backend |
| **Dev experience** | `docker-compose up` y funciona | Configurar env vars por ambiente |
| **Produccion** | Natural — ya hay un LB/proxy | Funciona pero requiere CORS mas permisivo |

Elegi Nginx porque **un build unico es mas confiable** — si funciona en staging, funciona en produccion. El costo (un container Nginx ligero) es negligible vs el beneficio de eliminar bugs por diferencias de ambiente.

### Mock como default LLM vs requerir API key

Elegi mock como default para que la aplicacion funcione out-of-the-box sin configuracion extra. Un desarrollador nuevo puede hacer `docker-compose up` y tener todo funcionando en 30 segundos. Si hubiera requerido API key, el primer contacto con la app seria un error 500 en clasificacion.

El trade-off es que el mock produce clasificaciones simplistas (keyword matching), pero para desarrollo y testing es suficiente. La activacion del LLM real es un cambio de env var (`LLM_PROVIDER=openai`).

### langchaingo vs SDK nativo de OpenAI

| | langchaingo | openai-go SDK |
|---|---|---|
| **Multi-provider** | Si (OpenAI, Ollama, Anthropic, etc.) | Solo OpenAI |
| **Madurez** | Comunidad pequena, hereda patterns de Python | SDK oficial, bien mantenido |
| **Abstraccion** | Interface unificada para cualquier LLM | API especifica de OpenAI |
| **Overhead** | Mas dependencias | Minimo |

Elegi langchaingo porque la interface `LLMClient` del proyecto requiere ser provider-agnostic. Con el SDK nativo, agregar Ollama o Anthropic requeriria reescribir el client. Con langchaingo, es un cambio en el factory.

### pgxpool directo vs repository pattern

Accedo a la BD directamente con `pgxpool` en los handlers, sin capa de repository. Es mas simple (KISS) y suficiente para el scope actual (~10 endpoints). El trade-off es que si el proyecto crece a 50+ endpoints con logica compleja, los handlers se volverian dificiles de testear y la duplicacion de queries seria un problema. En ese punto introduciria un repository pattern, pero no antes — seria abstraccion prematura.

### Next.js standalone vs server mode

Elegi `output: 'standalone'` para el Dockerfile porque produce una imagen mucho mas liviana (solo los archivos necesarios para servir, sin `node_modules` completo). El trade-off es que pierde acceso a algunos features de Next.js que dependen del filesystem completo (ISR con revalidacion on-demand, API routes complejas), pero este proyecto no los usa.

### Transaccion para clasificacion vs queries individuales

La persistencia de clasificacion (`persistClassification`) usa una transaccion de BD que agrupa el UPDATE de la tarea, DELETE de tags AI previos, y INSERT de nuevos tags. El trade-off es mayor complejidad y un lock mas largo en la fila. Pero sin transaccion, una falla a mitad del proceso dejaria la tarea en estado inconsistente (ej: tags viejos borrados pero nuevos no insertados).

---

## 6. Que Mejoraria con Mas Tiempo

Priorizado por impacto/esfuerzo:

### Inmediato (< 1 hora total)

1. **Fixear JWT exp** — Cambiar `fmt.Sprintf` por valor numerico directo. 5 minutos, restaura seguridad de autenticacion.
2. **Fixear edit history** — Capturar `oldStatus` antes de mutar. 10 minutos, restaura integridad de auditoria.
3. **Verificar error de UPDATE** — No descartar el error de `db.Pool.Exec`. 10 minutos, previene perdida silenciosa de datos.
4. **Activar validacion de password** — Agregar `bcrypt.CompareHashAndPassword`. 15 minutos, activa autenticacion real.

### Sprint actual (1-2 dias)

5. **Centralizar JWT secret** — Crear `internal/config` compartido entre handler y middleware. Elimina duplicacion y riesgo de desincronizacion.
6. **Graceful shutdown** — Implementar `http.Server.Shutdown()` con signal handling. Prerequisito para deploys sin downtime.
7. **Escapar metacaracteres LIKE** — Prevenir que `%` y `_` en busquedas retornen resultados incorrectos.
8. **Limitar tamano de request body** — `http.MaxBytesReader` como middleware global. Proteccion contra DoS.

### Proximo sprint (3-5 dias)

9. **Resolver N+1 queries** — Reemplazar queries individuales por JOINs para assignees y batch query para tags. Crucial para escalabilidad.
10. **Paginacion cursor-based** — En `ListTasks` y `SearchTasks`. Prerequisito para manejar miles de tareas.
11. **httpOnly cookies** — Migrar de localStorage a cookies seguras. Protege tokens contra XSS.
12. **Authorization por roles** — Middleware `RequireRole()` + verificacion de ownership en DELETE/UPDATE. El campo `role` actualmente es decorativo.

### Backlog

13. **Tests de integracion con testcontainers** — Reemplazar la dependencia de PostgreSQL local por containers efimeros. Tests mas confiables y reproducibles en CI.
14. **Auditoria completa** — Registrar historial de cambios para todos los campos, no solo status.
15. **Permitir actualizar due_date** — El campo existe en el modelo pero el handler lo ignora.
16. **Rate limiting en login** — Proteccion contra brute force (se vuelve critico al activar validacion de password).

---

## 7. Uso de IA (como herramienta de desarrollo)

### Herramienta utilizada

Use **Claude Code** (CLI de Anthropic) como unica herramienta de IA durante todo el desarrollo del proyecto.

### Para que lo use

| Area | Uso especifico | Modifique lo sugerido? |
|------|---------------|----------------------|
| **Code review** | Analisis exhaustivo del codebase existente. Claude identifico los 28 problemas documentados en REVIEW.md, los clasifico por severidad y propuso soluciones concretas con codigo. | Si — ajuste la priorizacion y redaccion para reflejar mi criterio. Las soluciones de codigo las valide contra la documentacion de las librerias. |
| **Integracion LLM** | Diseno de la interface `LLMClient`, implementacion del MockClient, LangChainClient y Factory. | Si — itere el prompt del LLM varias veces. La primera version era demasiado permisiva en el formato de respuesta. Agregue reglas explicitas y validacion server-side. |
| **Clasificacion handler** | Implementacion del endpoint `POST /tasks/{id}/classify` con transaccion para persistencia atomica. | Revision menor — ajuste el manejo de errores para ser mas especifico en los mensajes al cliente. |
| **Docker** | Dockerfiles multi-stage, nginx.conf, docker-compose.yml, y la arquitectura de proxy reverso. | Si — el primer intento de fix de UUIDs en el seed data fue un replace global que rompio nombres de tablas (`tags` → `fabs`). Lo detecte, reverti y hice un fix mas quirurgico. |
| **Testing** | Tests unitarios table-driven para MockClient, factory, middleware y handlers. | Ajustes menores en assertions y nombres de test cases para mayor claridad. |
| **Setup del proyecto** | CLAUDE.md, Makefile, golangci-lint config, Prettier config, git hooks. | Defini yo las reglas y convenciones; Claude las implemento en los archivos de configuracion. |

### Que aprendi del proceso

1. **La IA es buena para generar boilerplate pero hay que validar**: Los Dockerfiles multi-stage, la config de Nginx, y los tests table-driven salieron bien al primer intento. Pero operaciones como buscar-y-reemplazar requieren supervision — el replace global de `tag` por `fab` fue un ejemplo claro de donde la IA necesita guia humana.

2. **El prompt engineering aplica en ambas direcciones**: Asi como disene el prompt para el LLM de clasificacion, tambien aprendi a ser mas especifico en mis instrucciones a Claude Code. Instrucciones vagas producen resultados genericos; instrucciones precisas con contexto producen codigo que se integra bien con el proyecto existente.

3. **Code review asistido es valioso**: Hubiera sido facil pasar por alto el bug de `jwt exp` como string — funciona correctamente en desarrollo porque el token "no expira" de todos modos. Tener una revision sistematica que analiza cada linea ayudo a encontrar bugs sutiles que solo se manifiestan en produccion.
