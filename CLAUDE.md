# CLAUDE.md - Task Manager Project Guide

> Documento de referencia para desarrollo con Claude Code.
> Para decisiones de arquitectura detalladas, ver [ARCHITECTURE.md](./ARCHITECTURE.md).
> Para problemas identificados y su priorizacion, ver [REVIEW.md](./REVIEW.md).

---

## Reglas del Proyecto (OBLIGATORIAS)

Estas reglas aplican a TODO el codigo nuevo y modificado. No se aceptan excepciones sin aprobacion explicita.

### 1. KISS — Keep It Simple, Stupid

- Preferir la solucion mas simple que resuelva el problema.
- No agregar abstracciones, helpers ni utilidades para operaciones que se usan una sola vez.
- No sobredisenar: tres lineas de codigo repetidas son mejor que una abstraccion prematura.
- No agregar features, configurabilidad ni extensibilidad que no se haya pedido.
- Si una funcion necesita mas de 3 niveles de indentacion, simplificar con early returns.
- Evitar cleverness: el codigo debe ser legible por un junior dev en 30 segundos.

### 2. SOLID

- **S — Single Responsibility**: Cada funcion/struct/componente hace una sola cosa. Los handlers manejan HTTP, la logica de negocio va separada, los modelos solo definen datos.
- **O — Open/Closed**: Usar interfaces para extensibilidad (ej: `LLMClient` permite agregar proveedores sin modificar handlers). No modificar funciones existentes para agregar casos — extender.
- **L — Liskov Substitution**: Toda implementacion de una interface debe ser intercambiable sin romper el contrato. Si `MockClient` y `OpenAIClient` implementan `LLMClient`, ambos deben manejar errores de la misma forma.
- **I — Interface Segregation**: Interfaces pequenas y especificas. No crear interfaces "god" con 10+ metodos.
- **D — Dependency Inversion**: Los handlers dependen de interfaces, no de implementaciones concretas. Inyectar dependencias (DB pool, LLM client) en vez de acceder a variables globales.

### 3. Tests Obligatorios

**Todo codigo nuevo o modificado DEBE incluir tests que validen su comportamiento.**
**Todo archivo nuevo DEBE tener su archivo de test correspondiente junto al archivo fuente.**

#### Backend (Go)
- **Tests unitarios** (con mocks de BD): Para logica de negocio, validaciones, transformaciones de datos. Usan interfaces para mockear dependencias de BD.
- **Tests de integracion** (con testcontainers): Para queries SQL, flujos completos de API. Usan PostgreSQL real via testcontainers.
- **Ubicacion**: `backend/internal/<paquete>/*_test.go` — los tests viven junto al archivo que testean (Go best practice).
- **Package**: Usar black-box testing con sufijo `_test` en el paquete (ej: `package handler_test`) para testear solo la API publica.
- **Table-driven tests**: Usar subtests con `t.Run()` y tablas de test cases para cubrir multiples escenarios sin duplicar codigo.
- **Helpers**: Marcar funciones helper con `t.Helper()`. Usar `t.Cleanup()` para limpieza de recursos.
- **Paralelismo**: Agregar `t.Parallel()` en tests unitarios sin estado compartido.
- **Error messages**: Formato consistente: `FunctionName() field = %v, want %v`.
- Herramientas: `testing` package estandar de Go. Verificar interface compliance con `var _ Interface = (*Impl)(nil)`.
- Coverage minimo: toda funcion publica debe tener al menos un test de happy path y un test de error.

#### Frontend (TypeScript)
- Tests unitarios para logica de utilidades y transformaciones de datos.
- Tests de componentes para interacciones de UI criticas.
- **Ubicacion**: archivos `*.test.ts` o `*.test.tsx` junto al archivo que testean.

#### Que testear como minimo:
| Tipo de cambio | Tests requeridos |
|----------------|-----------------|
| Nuevo endpoint API | Unit test del handler + integration test del flujo completo |
| Nueva funcion de logica | Unit test con happy path + edge cases + errores |
| Nuevo componente React | Test de renderizado + test de interacciones clave |
| Bug fix | Test de regresion que reproduzca el bug original |
| Cambio de schema BD | Integration test que valide el schema + queries afectadas |

### 4. Clean Code

- **Nombres descriptivos**: Variables, funciones y archivos con nombres que explican su proposito. No abreviar innecesariamente (`taskID` no `tID`, `getUpcomingDeadlines` no `getUD`).
- **Funciones cortas**: Maximo ~30 lineas por funcion. Si crece mas, extraer subfunciones con nombres claros.
- **Un nivel de abstraccion por funcion**: No mezclar logica de alto nivel (orquestacion) con bajo nivel (parsing de strings) en la misma funcion.
- **No comentarios obvios**: El codigo debe ser auto-documentado. Solo comentar el "por que", nunca el "que". Excepciones: decisiones de negocio no evidentes, workarounds con referencia a issues.
- **Consistencia**: Seguir los patrones ya establecidos en el proyecto. Si el proyecto usa inline styles, no introducir CSS modules. Si usa maps para validacion, no cambiar a switch.
- **No dead code**: No dejar codigo comentado, imports no usados ni variables sin usar.
- **Error handling explicito**: En Go, siempre verificar errores. Nunca `_ = operacionCritica()`. En TypeScript, manejar todos los estados (loading, error, success).

### 5. Verificacion Pre-Commit (Obligatoria)

**Antes de cada commit se DEBE ejecutar y pasar exitosamente:**

```bash
# Ejecutar todo de una vez:
make check

# O paso a paso:
make lint          # Linting de Go + TypeScript
make test          # Tests unitarios + integracion
make security      # Verificacion de seguridad
```

#### Detalle de verificaciones:

| Paso | Backend (Go) | Frontend (TypeScript) |
|------|-------------|----------------------|
| **Lint** | `golangci-lint run ./...` | `npm run lint` (ESLint) + `npm run format:check` (Prettier) |
| **Tests** | `go test ./... -race` | `npm test` |
| **Security** | `golangci-lint` con gosec habilitado | ESLint security rules |
| **Build** | `go build ./...` | `npm run build` |

**Si cualquier paso falla, el commit NO debe realizarse.** Primero corregir los errores.

#### Git hook automatico
El proyecto incluye un git hook en `.githooks/pre-commit` que ejecuta estas verificaciones automaticamente. Para activarlo:
```bash
git config core.hooksPath .githooks
```

### 6. Conventional Commits (Obligatorio)

**Todos los commits DEBEN seguir la especificacion [Conventional Commits v1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).**

#### Formato
```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

#### Tipos permitidos
| Tipo | Descripcion |
|------|-------------|
| `feat` | Nueva funcionalidad |
| `fix` | Correccion de bug |
| `refactor` | Cambio de codigo que no agrega feature ni corrige bug |
| `test` | Agregar o modificar tests |
| `docs` | Cambios en documentacion |
| `style` | Formateo, linting, sin cambio de logica |
| `chore` | Cambios en tooling, config, dependencias |
| `perf` | Mejora de rendimiento |
| `ci` | Cambios en CI/CD |
| `build` | Cambios en sistema de build o dependencias externas |

#### Scopes del proyecto
- `backend`, `frontend`, `db`, `llm`, `auth`, `handler`, `middleware`, `config`

#### Reglas
- **Description**: En minusculas, imperativo, sin punto final. Max 72 caracteres.
- **Body**: Explicar el "por que" del cambio, no el "que". Separado por linea en blanco.
- **Breaking changes**: Usar `!` despues del tipo/scope o footer `BREAKING CHANGE:`.
- **Un commit = un cambio logico**: No mezclar features con refactors ni fixes con tests no relacionados.

#### Ejemplos
```
feat(llm): add POST /tasks/:id/classify endpoint

fix(auth): use numeric exp in JWT claims instead of string

refactor(handler): extract classify handler to separate file

test(llm): add table-driven tests for MockClient classification

chore(frontend): add eslint and prettier configuration

docs: add conventional commits rule to CLAUDE.md
```

---

## Descripcion del Proyecto

Aplicacion fullstack de gestion de tareas (Task Manager) para Kemeny Studio. Permite crear, asignar, clasificar y monitorear tareas con integracion de IA para auto-clasificacion.

## Stack Tecnologico

| Capa | Tecnologia | Version |
|------|-----------|---------|
| Backend | Go (chi router) | 1.22+ |
| Base de datos | PostgreSQL | 16 |
| ORM/Driver | pgx/v5 (pgxpool) | 5.7 |
| Auth | JWT (golang-jwt/v5) | 5.2 |
| Frontend | Next.js (App Router) | 14.2 |
| UI | React + TypeScript | 18.3 |
| IA | Interface LLMClient (OpenAI/Anthropic) | - |

## Estructura del Proyecto

```
├── backend/
│   ├── cmd/server/main.go              # Entry point, router setup, CORS
│   ├── internal/
│   │   ├── handler/
│   │   │   ├── tasks.go               # CRUD handlers + login + dashboard stats
│   │   │   ├── classify.go            # POST /tasks/{id}/classify handler
│   │   │   ├── classify_test.go       # Tests del classify handler
│   │   │   ├── update_task_test.go    # Tests del update task handler
│   │   │   └── testhelpers_test.go    # Helpers compartidos para tests de handler
│   │   ├── middleware/
│   │   │   ├── auth.go                # JWT auth middleware
│   │   │   └── auth_test.go           # Tests JWT claims y expiracion
│   │   ├── model/task.go              # Structs: Task, User, Tag, EditHistory, DTOs
│   │   ├── db/connection.go           # Pool PostgreSQL (pgxpool)
│   │   ├── notification/service.go    # Deadline notifications (stub)
│   │   └── llm/
│   │       ├── client.go              # Interface LLMClient + TaskClassification
│   │       ├── mock.go                # Mock client basado en keywords
│   │       ├── mock_test.go           # Tests del mock client (table-driven)
│   │       ├── langchain.go           # Client langchaingo multi-provider
│   │       ├── factory.go             # Factory: NewClient() por env var
│   │       └── factory_test.go        # Tests del factory
├── frontend/
│   └── src/
│       ├── app/
│       │   ├── layout.tsx             # Root layout con nav
│       │   ├── page.tsx               # Dashboard page
│       │   └── tasks/
│       │       ├── page.tsx           # Task list con filtros
│       │       └── [id]/page.tsx      # Task detail con cambio de status
│       ├── components/
│       │   ├── Dashboard.tsx          # Stats cards y breakdown
│       │   ├── TaskBoard.tsx          # Kanban board por columnas
│       │   └── TaskCard.tsx           # Card individual de tarea
│       ├── lib/api.ts                 # API client singleton (fetch + auth)
│       └── types/index.ts            # Interfaces TypeScript
├── database/
│   └── init.sql                       # Schema + seed data (4 users, 10 tasks, 8 tags)
├── ARCHITECTURE.md                    # Decisiones de arquitectura
└── README.md                          # Instrucciones del proyecto
```

## Comandos de Desarrollo

### Base de datos
```bash
# Levantar PostgreSQL
docker run -d --name taskdb \
  -e POSTGRES_PASSWORD=assessment \
  -e POSTGRES_DB=taskmanager \
  -p 5432:5432 \
  postgres:16-alpine

# Seed de datos
PGPASSWORD=assessment psql -h localhost -U postgres -d taskmanager -f database/init.sql
```

### Backend
```bash
cd backend
export DB_HOST=localhost DB_PORT=5432 DB_USER=postgres DB_PASSWORD=assessment DB_NAME=taskmanager
export JWT_SECRET=dev-secret
go run cmd/server/main.go              # Inicia en :8080
go test ./tests/...                     # Ejecutar tests
go test ./... -v                        # Tests verbose
go vet ./...                            # Analisis estatico
```

### Frontend
```bash
cd frontend
npm install                             # Instalar dependencias
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev    # Dev server :3000
npm run build                           # Build produccion
npm run lint                            # Linting
```

## API Endpoints

### Publicos
| Metodo | Ruta | Descripcion |
|--------|------|-------------|
| POST | `/api/auth/login` | Login (retorna JWT) |
| GET | `/health` | Health check |

### Protegidos (requieren `Authorization: Bearer <token>`)
| Metodo | Ruta | Descripcion |
|--------|------|-------------|
| GET | `/api/tasks` | Listar tareas (`?status=`, `?include=assignee`) |
| POST | `/api/tasks` | Crear tarea |
| GET | `/api/tasks/{id}` | Detalle de tarea |
| PUT | `/api/tasks/{id}` | Actualizar tarea |
| DELETE | `/api/tasks/{id}` | Eliminar tarea |
| GET | `/api/tasks/{id}/history` | Historial de ediciones |
| GET | `/api/tasks/search` | Buscar tareas (`?q=`) |
| GET | `/api/dashboard/stats` | Estadisticas del dashboard |

### Usuarios de prueba
| Email | Password | Rol |
|-------|----------|-----|
| carlos@kemeny.studio | password123 | admin |
| lucia@kemeny.studio | password123 | member |
| mateo@kemeny.studio | password123 | member |
| valentina@kemeny.studio | password123 | member |

> **Nota:** El login acepta cualquier password para usuarios del seed (bcrypt validation omitida intencionalmente).

## Modelo de Datos

### Tablas principales
- **users**: id (UUID), email, name, password_hash, role (admin/member)
- **tasks**: id (UUID), title, description, status (todo/in_progress/review/done), priority (low/medium/high/urgent), category, summary, creator_id, assignee_id, due_date, estimated_hours, actual_hours
- **tags**: id (UUID), name, color (hex)
- **task_tags**: task_id, tag_id, assigned_by (manual/ai)
- **edit_history**: id, task_id, user_id, field_name, old_value, new_value

### Relaciones
- Task → User (creator_id, assignee_id)
- Task ↔ Tag (many-to-many via task_tags)
- Task → EditHistory (one-to-many)

## Integracion IA (LLM)

### Interface
```go
// backend/internal/llm/client.go
type LLMClient interface {
    ClassifyTask(ctx context.Context, title string, description string) (*TaskClassification, error)
}

type TaskClassification struct {
    Tags     []string  // Tags sugeridos
    Priority string    // "high", "medium", "low"
    Category string    // "bug", "feature", "improvement", "research"
    Summary  string    // Resumen de una linea
}
```

### Endpoint pendiente de implementar
`POST /api/tasks/:id/classify` — Clasifica una tarea usando LLM y guarda resultado en BD.

## Patrones y Convenciones

### Backend (Go)
- **Router**: chi/v5 con middleware chain (Logger, Recoverer, RequestID, CORS, Auth)
- **DB**: Global `db.Pool` (*pgxpool.Pool) — acceso directo sin repository pattern
- **Auth**: JWT con claims `user_id`, `email`, `role` — secret via `JWT_SECRET` env var
- **Handlers**: Funciones en paquete `handler`, acceden a `db.Pool` directamente
- **Errores**: JSON strings inline (`{"error": "message"}`)
- **Validacion**: Inline en handlers con maps para valores validos
- **Config**: Variables de entorno con defaults via `getEnv(key, fallback)`

### Frontend (Next.js)
- **App Router**: Next.js 14 con `"use client"` en componentes interactivos
- **Estilos**: Inline styles (sin CSS modules ni Tailwind actualmente)
- **API Client**: Singleton `api` en `lib/api.ts` con token en localStorage
- **State**: useState/useEffect local (sin state management global)
- **Types**: Interfaces centralizadas en `types/index.ts`
- **Path aliases**: `@/*` → `./src/*`

## Problemas Conocidos

1. **Login sin validacion de password** — bcrypt comparison omitida (intencional para desarrollo)
2. **JWT exp como string** — `handler/tasks.go:547` guarda exp como string en vez de int64, puede causar problemas de validacion
3. **N+1 queries** — `ListTasks` hace query individual por cada assignee y tags de cada task
4. **No graceful shutdown** — El server termina abruptamente en SIGINT
5. **Update history bug** — `UpdateTask` registra el status viejo como old_value y new_value igual (usa `existing.Status` despues del update)
6. **Sin paginacion** — Todos los endpoints retornan todos los resultados
7. **CORS hardcoded** — Solo permite `localhost:3000`
8. **Dashboard sin auth real** — El frontend no maneja flujo de login

## Guia para Nuevas Funcionalidades

### Agregar un nuevo endpoint backend
1. Definir request/response structs en `backend/internal/model/task.go`
2. Crear handler en `backend/internal/handler/tasks.go` (una sola responsabilidad, validar input, manejar errores)
3. Registrar ruta en `backend/cmd/server/main.go` (dentro del grupo `/api` protegido)
4. **Escribir tests**: unit test con mock de BD + integration test con testcontainers
5. Ejecutar `make check` antes de commit

### Agregar un nuevo componente frontend
1. Crear componente en `frontend/src/components/NuevoComponente.tsx`
2. Agregar tipos necesarios en `frontend/src/types/index.ts`
3. Si requiere API calls, agregar metodo en `frontend/src/lib/api.ts`
4. Usar `"use client"` si el componente tiene state o event handlers
5. Importar con alias: `import { Component } from '@/components/Component'`
6. **Escribir tests**: test de renderizado + interacciones clave
7. Ejecutar `make check` antes de commit

### Agregar una nueva pagina
1. Crear directorio/archivo en `frontend/src/app/ruta/page.tsx`
2. Seguir patron de pages existentes (loading state, error handling, data fetching)
3. **Escribir tests** para el flujo de la pagina
4. Ejecutar `make check` antes de commit

### Implementar un nuevo LLM client
1. Crear archivo en `backend/internal/llm/` (ej: `openai.go`, `anthropic.go`)
2. Implementar interface `LLMClient` (Liskov: intercambiable con MockClient sin cambios en handlers)
3. Manejar: timeouts, retries, respuestas malformadas, rate limiting
4. Inyectar el client en el handler (Dependency Inversion, no variable global)
5. **Escribir tests**: unit test con respuestas mockeadas del provider + test de interface compliance
6. Ejecutar `make check` antes de commit

### Modificar esquema de base de datos
1. Agregar SQL en `database/init.sql` o crear archivo de migracion
2. Actualizar structs en `backend/internal/model/task.go`
3. Actualizar tipos en `frontend/src/types/index.ts`
4. Actualizar queries afectadas en `backend/internal/handler/tasks.go`
5. **Escribir tests**: integration test con testcontainers que valide schema + queries
6. Ejecutar `make check` antes de commit

---

## Skills para Claude Code

### Frontend Skill
Cuando trabajes en el frontend, sigue estas reglas:

**Reglas base (KISS + Clean Code):**
- Usar TypeScript estricto — no `any`, preferir interfaces sobre types
- Componentes client-side llevan `"use client"` al inicio
- Estilos inline (patron actual del proyecto) — no introducir CSS modules ni Tailwind sin aprobacion
- Funciones de componente cortas (~30 lineas max). Extraer subcomponentes si crece
- Nombres descriptivos: `handleStatusChange` no `handleChange`, `TaskDetailPage` no `TDP`
- No agregar props, estados ni features que no se pidieron
- No dejar console.log, imports sin usar ni codigo comentado

**SOLID aplicado a React:**
- Single Responsibility: un componente = una responsabilidad visual/funcional
- Open/Closed: componentes extensibles via props, no modificando el componente base
- Interface Segregation: props interfaces pequenas y especificas por componente
- Dependency Inversion: API calls a traves del singleton `api` en `lib/api.ts`, no fetch directo

**Patrones del proyecto:**
- Manejar estados: loading, error, y data en cada pagina/componente
- Tipos compartidos en `types/index.ts`
- Validar datos del API antes de renderizar (patron `validateStats` como type guard `is`)
- Links entre paginas con `<a href>` (patron actual, no `next/link`)
- Path alias `@/` para imports desde `src/`

**Tests obligatorios:**
- Todo componente nuevo debe tener test de renderizado
- Componentes con interaccion (clicks, forms) deben tener tests de eventos
- Logica extraida (helpers, transformaciones) debe tener unit tests
- Ejecutar `cd frontend && npm run lint && npm run format:check && npm test` antes de commit

**Linting y formateo:**
- ESLint: `npm run lint` — debe pasar sin errores ni warnings
- Prettier: `npm run format:check` — formateo consistente. Usar `npm run format` para auto-fix

### Backend Skill
Cuando trabajes en el backend, sigue estas reglas:

**Reglas base (KISS + Clean Code):**
- Go idiomatico: error handling explicito, no panic en handlers
- Funciones cortas (~30 lineas max). Si un handler crece, extraer logica a funciones helper
- Nombres descriptivos: `validateTaskStatus` no `valTS`, `getTaskByID` no `getT`
- No agregar abstracciones para cosas que se usan una vez
- No dead code: sin imports, variables ni funciones sin usar
- Early returns para reducir indentacion

**SOLID aplicado a Go:**
- Single Responsibility: handlers solo manejan HTTP (parse request, call logic, write response). Logica de negocio separada
- Open/Closed: nuevos LLM providers implementan `LLMClient` sin modificar handlers existentes
- Liskov Substitution: toda implementacion de `LLMClient` debe comportarse de forma intercambiable
- Interface Segregation: interfaces con 1-3 metodos. No interfaces "god"
- Dependency Inversion: inyectar dependencias (DB pool, LLM client) como parametros, no acceder a globales. Para codigo nuevo, recibir pool como argumento

**Patrones del proyecto:**
- Handlers como funciones exportadas en paquete `handler`
- SQL parametrizado siempre ($1, $2...) — NUNCA concatenar strings en queries
- Respuestas JSON con Content-Type header explicito
- Validacion de input inline en cada handler
- Context propagation: usar `r.Context()` en todas las queries
- Valores validos en maps: `map[string]bool{"value": true}`
- UserID del contexto via `middleware.GetUserID(r)`
- Logs con `log.Printf` para errores no fatales
- Nuevos modelos/structs en `model/task.go`

**Tests obligatorios:**
- **Unit tests** (con mocks): Para validaciones, logica de negocio, transformaciones. Mockear BD con interfaces
- **Integration tests** (con testcontainers): Para queries SQL y flujos completos. PostgreSQL real en container
- Ubicacion: `backend/tests/` o `backend/internal/<paquete>/*_test.go`
- Todo handler nuevo: al menos 1 happy path + 1 error path
- Todo bug fix: test de regresion que reproduzca el bug
- Verificar interface compliance: `var _ Interface = (*Impl)(nil)`

**Linting:**
- `golangci-lint run ./...` — debe pasar sin errores
- Linters activos: govet, staticcheck, errcheck, gosec, gocritic, revive
- Ejecutar antes de cada commit

### Code Review Skill
Al revisar codigo en este proyecto, verificar **todas** las secciones:

**1. Cumplimiento de reglas del proyecto:**
- [ ] KISS: la solucion es la mas simple posible? Hay abstracciones innecesarias?
- [ ] SOLID: cada funcion/componente tiene una sola responsabilidad? Se usan interfaces para extensibilidad?
- [ ] Tests: todo codigo nuevo tiene tests asociados? Coverage de happy path + errores?
- [ ] Clean Code: nombres descriptivos, funciones cortas, sin dead code, sin comentarios obvios?
- [ ] Pre-commit: pasan lint, tests, security checks?

**2. Seguridad:**
- [ ] SQL injection: queries siempre parametrizadas ($1, $2...), nunca concatenacion
- [ ] Auth: todos los endpoints protegidos pasan por `AuthMiddleware`
- [ ] JWT: claims correctos, exp como int64 (no string), secret no hardcoded
- [ ] Input validation: tamano de campos, valores permitidos, body size limitado
- [ ] No exponer password_hash en responses (tag `json:"-"`)
- [ ] LIKE queries: metacaracteres `%` y `_` escapados
- [ ] Request body: limitado con `http.MaxBytesReader`

**3. Calidad:**
- [ ] Error handling: NO ignorar errores con `_ =` en operaciones de escritura (INSERT, UPDATE, DELETE)
- [ ] Errores de Scan verificados y loggeados (no `_ = rows.Scan(...)` en datos criticos)
- [ ] N+1 queries: preferir JOINs o batch queries sobre queries en loop
- [ ] Consistencia en formato de errores JSON
- [ ] Null safety en campos opcionales (*string, *time.Time)
- [ ] Edit history: registrar old_value ANTES de mutar la variable

**4. Performance:**
- [ ] Queries optimizadas con indices existentes
- [ ] Evitar cargar datos innecesarios
- [ ] Paginacion en listados
- [ ] No operaciones O(n) dentro de loops O(n)

**5. Frontend:**
- [ ] TypeScript estricto: no `any`, interfaces correctas
- [ ] Loading/error states manejados en cada componente
- [ ] No memory leaks en useEffect (cleanup functions si hay subscriptions/timers)
- [ ] Datos del API validados antes de uso (type guards)
- [ ] useEffect dependencies completas (regla exhaustive-deps)

**6. Testing (BLOQUEANTE — no aprobar sin tests):**
- [ ] Todo codigo nuevo tiene tests asociados
- [ ] Tests unitarios con mocks para logica de negocio
- [ ] Tests de integracion con testcontainers para queries SQL
- [ ] Tests de regresion para bug fixes
- [ ] Interface compliance verificada
- [ ] Tests pasan localmente (`make check`)