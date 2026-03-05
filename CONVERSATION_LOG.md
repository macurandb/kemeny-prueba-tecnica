# Registro de Conversacion — Claude Code

> Registro del proceso de desarrollo asistido por IA (Claude Code) para el proyecto Task Manager.
> Cada sesion documenta las instrucciones del usuario, las acciones de Claude y los resultados obtenidos.

---

## Sesion 1: Code Review + Setup del Proyecto

### Contexto inicial
Se recibio un codebase existente de un Task Manager (Go + Next.js + PostgreSQL) y se pidio hacer un code review exhaustivo, configurar tooling y establecer las reglas del proyecto.

### Acciones realizadas

**1. Code Review (REVIEW.md)**

El usuario pidio una revision exhaustiva del codigo. Claude analizo todo el codebase y genero un reporte con 28 problemas identificados:

- 4 CRITICOS: JWT exp como string, edit history corrupto, UPDATE error descartado, login sin validacion de password
- 8 ALTOS: JWT secret hardcoded, sin autorizacion por roles, LIKE sin escapar, password de BD en codigo, SSL deshabilitado, token en localStorage, sin logout
- 9 MEDIOS: N+1 queries, errores de Scan ignorados, sin paginacion, sin graceful shutdown, route shadowing
- 7 BAJOS: Non-null assertion, useEffect deps, CORS hardcoded, json.Encode sin verificar, sin rate limiting, sin CSRF, sin body size limit

Cada problema incluye: archivo, linea, impacto de negocio, criticidad, tiempo de fix y solucion con codigo.

**2. Setup de Tooling**

Se configuraron las herramientas de desarrollo:

- `backend/.golangci.yml` — Config golangci-lint con linters: govet, staticcheck, errcheck, gosec, gocritic, revive
- `frontend/.prettierrc` — Config Prettier para formato consistente
- `frontend/.eslintrc.json` — ESLint con eslint-config-next
- `Makefile` — Comandos: `make check`, `make lint`, `make test`, `make security`, `make build`, `make format`, `make setup`
- `.githooks/pre-commit` — Hook automatico que ejecuta todas las verificaciones

**3. CLAUDE.md — Guia del proyecto**

Se creo el documento de referencia con:
- 5 reglas obligatorias: KISS, SOLID, Tests, Clean Code, Pre-commit checks
- Skills de frontend, backend y code review
- Estructura del proyecto, comandos, API endpoints, patrones y convenciones

### Commits generados
```
docs: add code review report (REVIEW.md)
chore(config): add linting, formatting and pre-commit tooling
```

---

## Sesion 2: Integracion de IA (LLM)

### Instruccion del usuario
> Implementar el plan de POST /api/tasks/:id/classify con auto-clasificacion usando IA

### Acciones realizadas

**1. Interface LLMClient + MockClient**

Se partio de la interface `LLMClient` existente y se mejoro el `MockClient`:
- Clasificacion por keywords con prioridad de categorias (improvement antes de bug)
- Tags por topico: backend, frontend, devops, security, testing, performance
- Escalacion de prioridad por keywords: "urgent", "critical", "crash" → high

**2. LangChainClient (langchaingo)**

Se creo el client usando langchaingo como libreria multi-provider:

```go
// Prompt con formato JSON explicito + reglas de validacion
const classifyPrompt = `You are a task classifier...
Respond with this exact JSON format: { "tags": [...], "priority": "...", "category": "...", "summary": "..." }
Rules: tags max 4, priority based on urgency, category exactly one, summary max 80 chars.`
```

Iteraciones del prompt:
1. Primera version: formato libre → parsing impredecible
2. Segunda version: JSON explicito con reglas → mejoro consistencia
3. Version final: agregado `extractJSON()` para markdown fences + `validateClassification()` con defaults seguros

Parametros: `temperature=0.2` (determinista), `max_tokens=256` (control de costos).

**3. Factory Pattern**

`NewClient()` lee `LLM_PROVIDER` env var:
- `openai` → langchaingo OpenAI (requiere `OPENAI_API_KEY`)
- `ollama` → langchaingo Ollama (local, sin API key)
- `mock` o vacio → MockClient (fallback seguro)

Si el provider falla al inicializar, cae automaticamente a mock con log de warning.

**4. ClassifyHandler**

Handler con inyeccion de dependencias (`LLMClient` + `pgxpool.Pool`):
1. Carga task de BD
2. Valida que tiene titulo
3. Llama LLM con timeout de 30s
4. Persiste en transaccion: UPDATE task + DELETE AI tags viejos + INSERT nuevos tags (ON CONFLICT)
5. Recarga task completa con relaciones
6. Retorna JSON

**5. Frontend**

- Metodo `classifyTask(id)` en el API client
- Boton "Classify with AI" / "Re-classify with AI" en el detalle de tarea
- Estados: loading spinner, error con mensaje, success con reload

### Problemas encontrados y resueltos

- `openai.New()` no acepta `WithTemperature` en construccion → se movio a call-time
- ESLint v9 incompatible con Next.js 14 → downgrade a ESLint v8
- Warnings de useEffect deps son pre-existentes, no bloqueantes

### Commits generados
```
feat(llm): add langchaingo client with multi-provider factory
feat(handler): add POST /tasks/{id}/classify endpoint
feat(frontend): add classify with AI button in task detail
```

---

## Sesion 3: Refactoring de Tests

### Instruccion del usuario
> Mueve los archivos de tests junto al archivo fuente. Analiza los tests y aplica buenas practicas de Go testing.

### Acciones realizadas

**Archivos movidos:**

| Antes | Despues | Paquete |
|-------|---------|---------|
| `tests/tasks_test.go` | `internal/llm/mock_test.go` | `llm_test` |
| `tests/langchain_client_test.go` | `internal/llm/factory_test.go` | `llm_test` |
| `tests/update_task_test.go` | `internal/handler/update_task_test.go` | `handler_test` |
| `tests/classify_handler_test.go` | `internal/handler/classify_test.go` | `handler_test` |
| `tests/jwt_test.go` | `internal/middleware/auth_test.go` | `middleware_test` |
| `tests/helpers_test.go` | `internal/handler/testhelpers_test.go` | `handler_test` |

**Buenas practicas aplicadas:**

1. **Black-box testing** — Sufijo `_test` en paquete (`llm_test`, `handler_test`) para testear solo API publica
2. **Table-driven tests** — `TestMockClient_ClassifyTask` (5 casos), `TestNewClient` (4 casos), `TestUpdateTask` (3 casos)
3. **`t.Parallel()`** — En tests unitarios sin estado compartido
4. **`t.Helper()`** — En todas las funciones helper
5. **`t.Cleanup()`** — Para restaurar estado de BD y cerrar pools
6. **`t.Run()`** — Subtests con nombres descriptivos
7. **Error format** — Consistente: `FunctionName() field = %v, want %v`

**Problema encontrado:** `t.Setenv` no puede usarse en tests paralelos → se removio `t.Parallel()` del test de factory que modifica env vars.

### Commits generados
```
test: add tests alongside source files with Go best practices
```

---

## Sesion 4: Reglas del Proyecto

### Instruccion del usuario
> Adiciona como regla que cada archivo nuevo debe tener tests. Los commits deben seguir Conventional Commits. Haz los commits de todo.

### Acciones realizadas

**Reglas agregadas a CLAUDE.md:**

1. **Regla 3 actualizada** — "Todo archivo nuevo DEBE tener su archivo de test correspondiente junto al archivo fuente" con guia de Go testing
2. **Regla 6 nueva** — Conventional Commits v1.0.0 con tipos, scopes del proyecto, formato y ejemplos

### Commits generados
```
docs: add conventional commits rule and update test guidelines
```

---

## Sesion 5: Docker + docker-compose

### Instruccion del usuario
> Crea Dockerfiles + docker-compose.yml para levantar todo con `docker-compose up`. Ten en cuenta el Makefile.

### Proceso de diseno

**Pregunta clave de Claude:**
> Como debe comunicarse el frontend con el backend? Opciones: exponer backend:8080 directamente, proxy via Next.js rewrites, o Nginx reverse proxy.

**Decision del usuario:** Nginx reverse proxy.

**Implicacion arquitectonica:** El frontend usa rutas relativas (`/api/tasks` en vez de `http://localhost:8080/api/tasks`). Mismo build para dev y prod. Sin necesidad de `NEXT_PUBLIC_API_URL`.

### Acciones realizadas

**Archivos creados:**

1. `backend/Dockerfile` — Multi-stage: golang:1.24-alpine → alpine:3.21 (~15MB)
2. `frontend/Dockerfile` — Multi-stage: node:20-alpine deps → builder → standalone runner
3. `nginx/nginx.conf` — Reverse proxy: `/api/*` y `/health` → backend, `/*` → frontend
4. `docker-compose.yml` — 4 servicios: db (postgres:16-alpine), backend, frontend, nginx
5. `backend/.dockerignore` + `frontend/.dockerignore`

**Archivos modificados:**

- `frontend/src/lib/api.ts` — Default API_URL cambiado de `http://localhost:8080` a `""` (relativo)
- `frontend/next.config.js` — Eliminado `NEXT_PUBLIC_API_URL` hardcoded
- `backend/cmd/server/main.go` — CORS configurable via `CORS_ORIGIN` env var + helper `getEnv()`
- `Makefile` — Targets: `docker-up`, `docker-down`, `docker-build`, `docker-logs`

### Verificacion end-to-end

```bash
docker-compose up --build -d
# Resultado: 4 containers running (db healthy, backend, frontend, nginx:80)

curl http://localhost/health
# → {"status": "ok"}

curl -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"carlos@kemeny.studio","password":"password123"}'
# → {"token":"eyJhbG...","user":{...}}

curl http://localhost/api/tasks -H "Authorization: Bearer $TOKEN"
# → 10 tasks

curl -X POST http://localhost/api/tasks/11111111.../classify -H "Authorization: Bearer $TOKEN"
# → Task clasificada con tags, priority, category, summary

curl http://localhost/
# → HTML del dashboard (Next.js)
```

### Bug encontrado y corregido

**UUID invalido en seed data:** Los tags usaban prefijo `tag` (ej: `tag11111-1111-...`) que no es hexadecimal valido. PostgreSQL rechazaba el INSERT durante `docker-entrypoint-initdb.d/init.sql`. Se reemplazo con prefijo `fab` (hex valido).

**Error durante el fix:** El primer intento uso `replace_all` de `tag` por `fab` en todo el archivo, lo que rompio nombres de tablas (`tags` → `fabs`) y strings (`staging` → `sfabing`). Se detecto inmediatamente, se revirtio con `git checkout`, y se hizo un reemplazo quirurgico solo en los valores UUID.

### Commits generados
```
fix(db): use valid hex prefix in tag seed UUIDs
refactor(frontend): use relative API URLs for proxy compatibility
feat(backend): make CORS origin configurable via env var
chore(docker): add Dockerfiles, nginx proxy and docker-compose
```

---

## Sesion 6: Documento de Arquitectura

### Instruccion del usuario
> Completa el archivo ARCHITECTURE.md respondiendo cada pregunta. Hazme las preguntas que necesites.

### Preguntas realizadas por Claude

1. **Uso de IA:** Solo Claude Code como herramienta de desarrollo
2. **Iteraciones del prompt LLM:** Multiples iteraciones. langchaingo elegido por herencia de LangChain Python + performance de Go
3. **Cloud provider:** AWS (ECS, RDS, ElastiCache, SQS, CloudFront)
4. **Tono:** Primera persona (prueba tecnica)

### Secciones completadas

1. **Code Review** — Top 3 problemas con justificacion de priorizacion (seguridad > datos > funcionalidad)
2. **Integracion de IA** — langchaingo, 3 iteraciones del prompt, manejo de fallos, costos (~$0.50/10K clasificaciones), plan de batch processing
3. **Docker y Orquestacion** — Nginx proxy, multi-stage builds, healthcheck, WebSocket headers, CORS configurable
4. **Arquitectura para Produccion** — Diagrama AWS: CloudFront → ALB → ECS Fargate → RDS Multi-AZ + Redis + SQS + Workers
5. **Trade-offs** — 6 decisiones documentadas: Nginx vs URL, mock default, langchaingo vs SDK nativo, pgxpool directo, standalone Next.js, transaccion para clasificacion
6. **Que Mejoraria** — 16 items priorizados en 4 niveles (inmediato → backlog)
7. **Uso de IA** — Claude Code para code review, LLM integration, Docker, testing. Incluyendo errores y como se corrigieron

---

## Resumen de Commits (orden cronologico)

```
docs: add code review report (REVIEW.md)
chore(config): add linting, formatting and pre-commit tooling
fix(backend): use numeric exp in JWT claims, fix edit history, check UPDATE error
fix(llm): mock client improvement classification ordering
feat(llm): add langchaingo client with multi-provider factory
feat(handler): add POST /tasks/{id}/classify endpoint
feat(frontend): add classify with AI button in task detail
test: add tests alongside source files with Go best practices
docs: add conventional commits rule and update test guidelines
fix(db): use valid hex prefix in tag seed UUIDs
refactor(frontend): use relative API URLs for proxy compatibility
feat(backend): make CORS origin configurable via env var
chore(docker): add Dockerfiles, nginx proxy and docker-compose
docs: complete ARCHITECTURE.md with architectural decisions
```

## Archivos Creados/Modificados (totales)

### Nuevos
| Archivo | Descripcion |
|---------|-------------|
| `REVIEW.md` | Code review con 28 issues |
| `ARCHITECTURE.md` | Decisiones de arquitectura |
| `CONVERSATION_LOG.md` | Este documento |
| `Makefile` | Comandos de desarrollo + Docker |
| `.githooks/pre-commit` | Hook pre-commit automatico |
| `backend/.golangci.yml` | Config linter Go |
| `backend/Dockerfile` | Multi-stage build Go |
| `backend/.dockerignore` | Exclusiones Docker |
| `backend/internal/llm/langchain.go` | Client langchaingo |
| `backend/internal/llm/factory.go` | Factory multi-provider |
| `backend/internal/llm/mock_test.go` | Tests MockClient |
| `backend/internal/llm/factory_test.go` | Tests Factory |
| `backend/internal/handler/classify.go` | Handler clasificacion |
| `backend/internal/handler/classify_test.go` | Tests clasificacion |
| `backend/internal/handler/update_task_test.go` | Tests UpdateTask |
| `backend/internal/handler/testhelpers_test.go` | Helpers de test |
| `backend/internal/middleware/auth_test.go` | Tests JWT |
| `frontend/Dockerfile` | Multi-stage build Next.js |
| `frontend/.dockerignore` | Exclusiones Docker |
| `frontend/.eslintrc.json` | Config ESLint |
| `frontend/.prettierrc` | Config Prettier |
| `frontend/public/.gitkeep` | Placeholder para Docker |
| `nginx/nginx.conf` | Config reverse proxy |
| `docker-compose.yml` | Orquestacion de servicios |

### Modificados
| Archivo | Cambio |
|---------|--------|
| `CLAUDE.md` | Reglas KISS, SOLID, tests, clean code, pre-commit, conventional commits |
| `backend/cmd/server/main.go` | Ruta classify, CORS configurable, getEnv |
| `backend/internal/handler/tasks.go` | Fix JWT exp, edit history, UPDATE error |
| `backend/internal/llm/mock.go` | Mejora ordering de clasificacion |
| `backend/go.mod` / `go.sum` | Dependencia langchaingo |
| `frontend/src/lib/api.ts` | classifyTask(), API_URL relativo |
| `frontend/src/app/tasks/[id]/page.tsx` | Boton classify con AI |
| `frontend/next.config.js` | Removido env hardcoded |
| `frontend/package.json` | DevDeps: eslint, prettier |
| `database/init.sql` | Fix UUIDs de tags (tag→fab) |
