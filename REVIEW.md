# Code Review — Task Manager

> Revision exhaustiva del codigo fuente. Problemas ordenados por severidad (critico → bajo).
> Cada item incluye: problema, impacto de negocio, tiempo estimado de fix y valor aportado.

---

## Resumen Ejecutivo

Se identificaron **28 problemas** en el codebase:

| Severidad | Cantidad | Descripcion |
|-----------|----------|-------------|
| CRITICO | 4 | Bugs que causan perdida de datos o anulan la seguridad |
| ALTO | 8 | Vulnerabilidades de seguridad y fallos arquitecturales |
| MEDIO | 9 | Problemas de rendimiento, datos incompletos y robustez |
| BAJO | 7 | Mejoras de calidad de codigo y hardening |

**Top 3 problemas mas graves:**
1. **Tokens JWT nunca expiran** — el claim `exp` se guarda como string en vez de numero, lo que anula la validacion de expiracion
2. **Edit history corrupto** — registra el valor nuevo como viejo y nuevo, perdiendo el dato original
3. **UPDATE silencioso** — si falla la actualizacion de una tarea, el error se ignora y se responde como si hubiera funcionado

---

## CRITICOS

### 1. JWT `exp` como string — Tokens nunca expiran

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:547` |
| **Problema** | `fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix())` genera un **string** `"1741209600"` en vez de un **numero** `1741209600`. El RFC 7519 requiere que `exp` sea un NumericDate (JSON number). La libreria `golang-jwt/v5` intenta leer `exp` como `float64`; al encontrar un string, la asercion de tipo falla silenciosamente y **la validacion de expiracion se omite**. |
| **Impacto** | Todo token emitido es valido **para siempre**. Un token robado no puede ser revocado por tiempo. Si un empleado deja la empresa o un token se filtra en logs, el acceso persiste indefinidamente. |
| **Criticidad** | CRITICA — Vulnerabilidad de seguridad que invalida toda la autenticacion |
| **Tiempo de fix** | 5 minutos |
| **Valor** | Restaura la seguridad fundamental del sistema de autenticacion |

**Solucion recomendada:**
```go
// ANTES (bug):
"exp": fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),

// DESPUES (fix):
"exp": time.Now().Add(24 * time.Hour).Unix(),
```

---

### 2. Edit History registra old_value incorrecto

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:305, 337-341` |
| **Problema** | El handler primero muta `existing.Status = *req.Status` (linea 305), y despues registra el historial usando `existing.Status` como `old_value` (linea 339). En ese punto, `existing.Status` ya contiene el valor **nuevo**, asi que `old_value` y `new_value` son identicos. Se pierde el valor original. |
| **Impacto** | La tabla `edit_history` contiene datos corruptos. Cualquier funcionalidad de auditoria, rollback o compliance que dependa del historial esta comprometida. No se puede reconstruir el estado anterior de una tarea. |
| **Criticidad** | CRITICA — Perdida silenciosa de datos de auditoria |
| **Tiempo de fix** | 10 minutos |
| **Valor** | Restaura la integridad del historial de cambios, esencial para auditoria y trazabilidad |

**Solucion recomendada:**
```go
// Capturar valor original ANTES de mutar
oldStatus := existing.Status

if req.Status != nil {
    // ... validacion ...
    existing.Status = *req.Status
}

// ... UPDATE query ...

// Registrar historial con el valor original
if req.Status != nil {
    _, err := db.Pool.Exec(r.Context(),
        `INSERT INTO edit_history (task_id, user_id, field_name, old_value, new_value)
         VALUES ($1, $2, 'status', $3, $4)`,
        taskID, userID, oldStatus, *req.Status,
    )
    if err != nil {
        log.Printf("error recording edit history: %v", err)
    }
}
```

---

### 3. Error de UPDATE descartado silenciosamente

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:325` |
| **Problema** | `_, _ = db.Pool.Exec(r.Context(), "UPDATE tasks SET ...")` — tanto el `CommandTag` como el `error` se descartan con `_, _`. Si el UPDATE falla por cualquier razon (constraint violation, conexion perdida, deadlock), el handler continua, re-lee la fila sin cambios y la retorna al cliente como si la actualizacion hubiera sido exitosa. |
| **Impacto** | El usuario cree que su cambio se guardo, pero no fue asi. Puede causar perdida de trabajo del usuario, inconsistencias en el estado del proyecto y perdida de confianza en la aplicacion. |
| **Criticidad** | CRITICA — Perdida silenciosa de datos del usuario |
| **Tiempo de fix** | 10 minutos |
| **Valor** | Garantiza que los errores de escritura se detecten y comuniquen al usuario |

**Solucion recomendada:**
```go
result, err := db.Pool.Exec(r.Context(),
    `UPDATE tasks SET title=$1, description=$2, status=$3, priority=$4,
     assignee_id=$5, estimated_hours=$6, actual_hours=$7, updated_at=NOW()
     WHERE id=$8`,
    existing.Title, existing.Description, existing.Status, existing.Priority,
    existing.AssigneeID, existing.EstimatedHours, existing.ActualHours,
    taskID,
)
if err != nil {
    log.Printf("error updating task %s: %v", taskID, err)
    http.Error(w, `{"error": "failed to update task"}`, http.StatusInternalServerError)
    return
}
if result.RowsAffected() == 0 {
    http.Error(w, `{"error": "task not found"}`, http.StatusNotFound)
    return
}
```

---

### 4. Login sin validacion de password

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:537-540` |
| **Problema** | El password hash se carga de la BD pero se descarta explicitamente con `_ = user.PasswordHash`. Cualquier request con un email valido y **cualquier** password recibe un JWT valido. El comentario indica que es intencional para el assessment, pero el seed data ya contiene bcrypt hashes validos. |
| **Impacto** | La autenticacion es una ilusion. Cualquier persona que conozca un email de usuario tiene acceso completo al sistema. En produccion, esto seria una brecha total de seguridad. |
| **Criticidad** | CRITICA — Sin autenticacion real (intencional segun comentario, pero sigue siendo critico) |
| **Tiempo de fix** | 15 minutos |
| **Valor** | Activa la autenticacion real del sistema, prerequisito para cualquier despliegue |

**Solucion recomendada:**
```go
import "golang.org/x/crypto/bcrypt"

// Reemplazar:
//   _ = user.PasswordHash
// Con:
if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
    http.Error(w, `{"error": "invalid credentials"}`, http.StatusUnauthorized)
    return
}
```

---

## ALTO

### 5. JWT secret hardcoded como fallback

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:24-28` y `backend/internal/middleware/auth.go:18-23` |
| **Problema** | Ambos archivos usan el fallback `"default-secret-change-in-production"` cuando `JWT_SECRET` no esta definido. No hay warning ni fallo al arranque. |
| **Impacto** | Si se despliega sin configurar el env var, cualquier persona que lea el codigo fuente puede forjar tokens JWT validos y suplantar a cualquier usuario. |
| **Criticidad** | ALTA — Backdoor involuntario en produccion |
| **Tiempo de fix** | 15 minutos |
| **Valor** | Elimina el riesgo de despliegue inseguro por omision de configuracion |

**Solucion recomendada:**
```go
func init() {
    secret := os.Getenv("JWT_SECRET")
    if secret == "" {
        log.Fatal("JWT_SECRET environment variable is required")
    }
    jwtSecret = []byte(secret)
}
```

---

### 6. JWT secret duplicado — riesgo de desincronizacion

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:21` y `backend/internal/middleware/auth.go:16` |
| **Problema** | Dos paquetes declaran su propio `var jwtSecret []byte` con su propio `init()`. El orden de ejecucion de `init()` entre paquetes en Go depende del orden de importacion y no esta garantizado. |
| **Impacto** | Si los dos `init()` leen el env var en momentos diferentes, podrian usar secrets distintos: tokens generados en el handler serian rechazados por el middleware o viceversa. |
| **Criticidad** | ALTA — Fallo intermitente de autenticacion dificil de diagnosticar |
| **Tiempo de fix** | 20 minutos |
| **Valor** | Elimina fuente de bugs intermitentes, mejora mantenibilidad del codigo |

**Solucion recomendada:**

Crear un paquete compartido `internal/config/config.go`:
```go
package config

import (
    "log"
    "os"
)

var JWTSecret []byte

func Init() {
    secret := os.Getenv("JWT_SECRET")
    if secret == "" {
        log.Fatal("JWT_SECRET environment variable is required")
    }
    JWTSecret = []byte(secret)
}
```

Usar `config.JWTSecret` en ambos paquetes y llamar `config.Init()` en `main()`.

---

### 7. Sin control de acceso / autorizacion en endpoints

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go` (completo) y `backend/cmd/server/main.go:54-70` |
| **Problema** | Todo usuario autenticado puede: eliminar cualquier tarea, actualizar cualquier tarea, ver todas las tareas. El campo `role` (admin/member) del JWT nunca se verifica. La distincion admin/member es puramente decorativa. |
| **Impacto** | Un usuario `member` puede borrar tareas criticas de otros usuarios. No hay proteccion contra acciones maliciosas o accidentales entre miembros del equipo. |
| **Criticidad** | ALTA — Cualquier usuario puede modificar/eliminar datos de otros |
| **Tiempo de fix** | 1-2 horas |
| **Valor** | Protege la integridad de los datos del equipo, permite control granular de permisos |

**Solucion recomendada:**
```go
// Middleware de autorizacion por rol
func RequireRole(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userRole := GetUserRole(r) // extraer del context
            for _, role := range roles {
                if userRole == role {
                    next.ServeHTTP(w, r)
                    return
                }
            }
            http.Error(w, `{"error": "forbidden"}`, http.StatusForbidden)
        })
    }
}

// En DeleteTask, verificar ownership o rol admin:
if task.CreatorID != userID && userRole != "admin" {
    http.Error(w, `{"error": "forbidden"}`, http.StatusForbidden)
    return
}
```

---

### 8. Metacaracteres LIKE sin escapar en busqueda

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:421` |
| **Problema** | `searchTerm := "%" + strings.ToLower(q) + "%"` — los caracteres `%` y `_` en el input del usuario no se escapan. No es SQL injection (usa `$1`), pero es un bug logico: buscar `100%` o `_admin` retorna resultados inesperados. |
| **Impacto** | Resultados de busqueda incorrectos que confunden al usuario. Busquedas con `%` retornan practicamente todos los registros. |
| **Criticidad** | ALTA — Funcionalidad core rota para ciertos inputs |
| **Tiempo de fix** | 10 minutos |
| **Valor** | Busqueda confiable para todos los patrones de texto |

**Solucion recomendada:**
```go
// Escapar metacaracteres LIKE
q = strings.ReplaceAll(q, `\`, `\\`)
q = strings.ReplaceAll(q, `%`, `\%`)
q = strings.ReplaceAll(q, `_`, `\_`)
searchTerm := "%" + strings.ToLower(q) + "%"

// Agregar ESCAPE en la query:
`WHERE LOWER(title) LIKE $1 ESCAPE '\' OR LOWER(COALESCE(description, '')) LIKE $1 ESCAPE '\'`
```

---

### 9. Password de BD hardcoded en codigo fuente

| | |
|---|---|
| **Archivo** | `backend/internal/db/connection.go:17` |
| **Problema** | `password := getEnv("DB_PASSWORD", "assessment")` — el password por defecto esta en el codigo fuente committedo al repositorio. |
| **Impacto** | Cualquier persona con acceso al repositorio conoce las credenciales de la base de datos. Combinado con `sslmode=disable`, un atacante en la misma red puede conectarse directamente. |
| **Criticidad** | ALTA — Credenciales expuestas en version control |
| **Tiempo de fix** | 10 minutos |
| **Valor** | Elimina credenciales del codigo fuente, previene acceso no autorizado a BD |

**Solucion recomendada:**
```go
func Connect() error {
    password := os.Getenv("DB_PASSWORD")
    if password == "" {
        return fmt.Errorf("DB_PASSWORD environment variable is required")
    }
    // ... resto igual
}
```

---

### 10. Conexion a BD sin SSL

| | |
|---|---|
| **Archivo** | `backend/internal/db/connection.go:20` |
| **Problema** | `sslmode=disable` — todo el trafico de BD (queries, credenciales, datos) viaja sin cifrar. |
| **Impacto** | En redes compartidas o cloud, las credenciales y datos de usuarios son visibles en texto plano para cualquier interceptor de trafico. |
| **Criticidad** | ALTA — Datos sensibles transmitidos sin cifrar |
| **Tiempo de fix** | 15 minutos |
| **Valor** | Protege datos en transito, cumplimiento basico de seguridad |

**Solucion recomendada:**
```go
sslmode := getEnv("DB_SSLMODE", "require") // default seguro
dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
    user, password, host, port, dbname, sslmode)
```

---

### 11. Token JWT almacenado en localStorage

| | |
|---|---|
| **Archivo** | `frontend/src/lib/api.ts:10-12` |
| **Problema** | `localStorage.setItem('auth_token', token)` — `localStorage` es accesible por cualquier JavaScript en el mismo origen. Si existe cualquier vulnerabilidad XSS (script de terceros, dependencia comprometida, futuro uso de `dangerouslySetInnerHTML`), el atacante roba el token. |
| **Impacto** | Un ataque XSS permite robar sesiones de todos los usuarios afectados. Combinado con tokens sin expiracion (#1), el acceso robado es permanente. |
| **Criticidad** | ALTA — Superficie de ataque amplificada por XSS |
| **Tiempo de fix** | 1-2 horas |
| **Valor** | Proteccion de sesiones contra XSS, reduccion significativa de riesgo |

**Solucion recomendada:**

Usar cookies `httpOnly` con `SameSite=Strict`:
```go
// Backend: setear cookie en vez de retornar token en JSON
http.SetCookie(w, &http.Cookie{
    Name:     "auth_token",
    Value:    tokenString,
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteStrictMode,
    Path:     "/",
    MaxAge:   86400, // 24 horas
})
```

```typescript
// Frontend: enviar credenciales con cookies
const response = await fetch(`${API_URL}${path}`, {
    ...options,
    credentials: 'include',
    headers,
});
```

---

### 12. Sin mecanismo de logout / invalidacion de token

| | |
|---|---|
| **Archivo** | `frontend/src/lib/api.ts` (completo) |
| **Problema** | `ApiClient` tiene `setToken()` y `getToken()` pero no tiene `logout()` ni `clearToken()`. No hay forma de invalidar una sesion. |
| **Impacto** | El usuario no puede cerrar sesion de forma segura. Si se cambia de dispositivo o sospecha que su cuenta fue comprometida, no puede protegerse. |
| **Criticidad** | ALTA — Funcionalidad basica de seguridad ausente |
| **Tiempo de fix** | 30 minutos |
| **Valor** | Control de sesion para el usuario, requisito basico de UX de seguridad |

**Solucion recomendada:**
```typescript
// En ApiClient:
logout() {
    this.token = null;
    if (typeof window !== 'undefined') {
        localStorage.removeItem('auth_token');
    }
}

// Para invalidacion server-side (futuro), considerar:
// - Token blacklist en Redis
// - Refresh token rotation
// - Reducir TTL del JWT a 15 min + refresh token de 7 dias
```

---

## MEDIO

### 13. N+1 queries en ListTasks

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:70-103` |
| **Problema** | Para cada tarea se ejecutan 1-2 queries adicionales: una para el assignee (lineas 70-83) y otra para los tags (lineas 86-103). Con 100 tareas = 200+ queries adicionales. |
| **Impacto** | Tiempos de respuesta degradados exponencialmente con el crecimiento de datos. Con 1000+ tareas, el endpoint se vuelve inutilizable (el propio seed data reconoce esto en la tarea #66666666). |
| **Criticidad** | MEDIA — Degradacion progresiva de performance |
| **Tiempo de fix** | 1-2 horas |
| **Valor** | Endpoint escalable, tiempos de respuesta consistentes independiente del volumen |

**Solucion recomendada:**
```go
// Reemplazar queries individuales con JOINs o batch queries:

// Opcion 1: JOIN para assignees
query := `SELECT t.*, u.id, u.email, u.name, u.role, u.avatar_url
          FROM tasks t
          LEFT JOIN users u ON t.assignee_id = u.id
          ORDER BY t.created_at DESC`

// Opcion 2: Batch query para tags (una sola query para todos los task_ids)
taskIDs := extractIDs(tasks)
tagRows, _ := db.Pool.Query(ctx,
    `SELECT tt.task_id, t.id, t.name, t.color, t.created_at
     FROM tags t
     INNER JOIN task_tags tt ON t.id = tt.tag_id
     WHERE tt.task_id = ANY($1)`, taskIDs)
// Agrupar tags por task_id en un map
```

---

### 14. Errores de Scan ignorados silenciosamente

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:98, 137, 147, 163, 405, 487, 501` |
| **Problema** | Multiples ubicaciones usan `_ = rows.Scan(...)`. Si el schema de BD cambia o el orden de columnas no coincide, se producen structs con zero-values sin indicacion de fallo. |
| **Impacto** | Datos incompletos o vacios enviados al frontend sin advertencia. Debugging dificil porque no hay logs que indiquen el problema. |
| **Criticidad** | MEDIA — Datos silenciosamente incorrectos |
| **Tiempo de fix** | 30 minutos |
| **Valor** | Deteccion rapida de problemas de schema, datos confiables |

**Solucion recomendada:**
```go
// Reemplazar _ = rows.Scan(...) con:
if err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt); err != nil {
    log.Printf("error scanning tag for task %s: %v", t.ID, err)
    continue
}
```

---

### 15. Creator siempre asignado aunque falle el query

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:136-141` |
| **Problema** | `_ = db.Pool.QueryRow(...).Scan(&creator...)` seguido de `t.Creator = &creator` sin verificar error. Si el usuario fue eliminado o el query falla, `t.Creator` apunta a un `User` con strings vacios. |
| **Impacto** | El frontend muestra datos vacios o "Unknown" en vez de `null`, confundiendo al usuario y dificultando la deteccion de datos huerfanos. |
| **Criticidad** | MEDIA — Datos enganiosos en la UI |
| **Tiempo de fix** | 10 minutos |
| **Valor** | Datos precisos en la API, mejor UX |

**Solucion recomendada:**
```go
var creator model.User
err = db.Pool.QueryRow(r.Context(),
    "SELECT id, email, name, role, avatar_url, created_at, updated_at FROM users WHERE id = $1",
    t.CreatorID,
).Scan(&creator.ID, &creator.Email, &creator.Name, &creator.Role, &creator.AvatarURL, &creator.CreatedAt, &creator.UpdatedAt)
if err == nil {
    t.Creator = &creator
}
// Si falla, t.Creator queda nil (correcto semanticamente)
```

---

### 16. Sin paginacion en ListTasks y SearchTasks

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:32-107, 414-455` |
| **Problema** | Ambos endpoints retornan todos los resultados sin `LIMIT`. No hay parametros de paginacion. |
| **Impacto** | Con miles de tareas: uso excesivo de memoria en el server, payloads enormes para el cliente, tiempos de respuesta inaceptables. Potencial vector de DoS. |
| **Criticidad** | MEDIA — Escalabilidad bloqueada |
| **Tiempo de fix** | 1-2 horas |
| **Valor** | API escalable, proteccion contra abuso de recursos |

**Solucion recomendada:**
```go
// Cursor-based pagination (recomendado sobre offset):
limit := 20
cursor := r.URL.Query().Get("cursor") // UUID del ultimo task visto

query := `SELECT ... FROM tasks WHERE created_at < $1 ORDER BY created_at DESC LIMIT $2`
// Retornar next_cursor en la respuesta para que el cliente pida la siguiente pagina
```

---

### 17. Sin limite de longitud en query de busqueda

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:415-419` |
| **Problema** | El parametro `q` se verifica por vacio pero no por longitud maxima. Un atacante puede enviar megabytes de texto, consumiendo memoria y generando operaciones LIKE costosas. |
| **Impacto** | Vector de DoS: queries lentas que bloquean conexiones del pool de BD. |
| **Criticidad** | MEDIA — Riesgo de degradacion de servicio |
| **Tiempo de fix** | 5 minutos |
| **Valor** | Proteccion contra abuso del endpoint de busqueda |

**Solucion recomendada:**
```go
if q == "" {
    http.Error(w, `{"error": "query parameter q is required"}`, http.StatusBadRequest)
    return
}
if len(q) > 200 {
    http.Error(w, `{"error": "query too long, max 200 characters"}`, http.StatusBadRequest)
    return
}
```

---

### 18. Sin graceful shutdown

| | |
|---|---|
| **Archivo** | `backend/cmd/server/main.go:81` |
| **Problema** | `http.ListenAndServe` no permite shutdown controlado. En SIGINT/SIGTERM, requests en vuelo se cortan abruptamente, conexiones de BD pueden quedar abiertas, transacciones quedan inconsistentes. |
| **Impacto** | En deploys (rolling updates, redeploys), usuarios activos pierden sus requests. Posible corrupcion de datos si se interrumpe una transaccion. |
| **Criticidad** | MEDIA — Riesgo de perdida de datos en deploys |
| **Tiempo de fix** | 30 minutos |
| **Valor** | Zero-downtime deploys, proteccion de datos en transito |

**Solucion recomendada:**
```go
srv := &http.Server{Addr: addr, Handler: r}

go func() {
    if err := srv.ListenAndServe(); err != http.ErrServerClosed {
        log.Fatalf("Server failed: %v", err)
    }
}()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

log.Println("Shutting down server...")
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
    log.Fatalf("Server forced to shutdown: %v", err)
}
log.Println("Server exited cleanly")
```

---

### 19. Riesgo de route shadowing: `/tasks/search` vs `/tasks/{id}`

| | |
|---|---|
| **Archivo** | `backend/cmd/server/main.go:60, 66` |
| **Problema** | Chi router maneja la precedencia de rutas literales sobre parametrizadas, pero el orden de registro (`{id}` antes de `search`) depende del comportamiento interno del router. Si se cambia de router o se reordena, `search` podria interpretarse como un `id`. |
| **Impacto** | Fragilidad: un refactor de rutas podria romper la busqueda sin advertencia. |
| **Criticidad** | MEDIA — Riesgo de regresion |
| **Tiempo de fix** | 5 minutos |
| **Valor** | Codigo robusto e independiente de implementacion interna del router |

**Solucion recomendada:**
```go
// Registrar la ruta literal ANTES de la parametrizada:
r.Get("/tasks/search", handler.SearchTasks)  // literal primero
r.Get("/tasks/{id}", handler.GetTask)        // parametrizada despues
```

---

### 20. Solo cambios de status se registran en edit_history

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:336-342` |
| **Problema** | `UpdateTask` solo registra historial para `status`. Cambios en `title`, `description`, `priority`, `assignee_id`, `estimated_hours` y `actual_hours` no se registran, a pesar de que la tabla `edit_history` soporta cualquier campo. |
| **Impacto** | Auditoria incompleta. No se puede saber quien cambio el titulo, prioridad o asignacion de una tarea, ni cuando. |
| **Criticidad** | MEDIA — Trazabilidad parcial |
| **Tiempo de fix** | 30 minutos |
| **Valor** | Auditoria completa de todos los cambios, accountability del equipo |

**Solucion recomendada:**
```go
// Funcion helper para registrar cambios:
func recordHistory(ctx context.Context, taskID, userID, field, oldVal, newVal string) {
    if oldVal == newVal {
        return
    }
    _, err := db.Pool.Exec(ctx,
        `INSERT INTO edit_history (task_id, user_id, field_name, old_value, new_value)
         VALUES ($1, $2, $3, $4, $5)`,
        taskID, userID, field, oldVal, newVal,
    )
    if err != nil {
        log.Printf("error recording history for task %s field %s: %v", taskID, field, err)
    }
}

// Llamar para cada campo que cambie:
if req.Title != nil && *req.Title != existing.Title {
    recordHistory(r.Context(), taskID, userID, "title", existing.Title, *req.Title)
}
if req.Priority != nil && *req.Priority != existing.Priority {
    recordHistory(r.Context(), taskID, userID, "priority", existing.Priority, *req.Priority)
}
// ... etc para cada campo
```

---

### 21. `due_date` no se puede actualizar via API

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:262-332` |
| **Problema** | `UpdateTaskRequest` en el modelo incluye `DueDate *string`, pero el handler `UpdateTask` nunca verifica `req.DueDate` y el SQL del UPDATE no incluye `due_date` en el SET. |
| **Impacto** | Los usuarios no pueden modificar la fecha limite de tareas existentes. Un workaround requeriria eliminar y recrear la tarea, perdiendo historial. |
| **Criticidad** | MEDIA — Funcionalidad de negocio faltante |
| **Tiempo de fix** | 20 minutos |
| **Valor** | Funcionalidad completa de gestion de tareas |

**Solucion recomendada:**
```go
// En UpdateTask, agregar manejo de due_date:
var dueDate *time.Time
if req.DueDate != nil {
    if *req.DueDate == "" {
        dueDate = nil // Permitir limpiar la fecha
    } else {
        parsed, err := time.Parse(time.RFC3339, *req.DueDate)
        if err != nil {
            http.Error(w, `{"error": "invalid due_date format"}`, http.StatusBadRequest)
            return
        }
        dueDate = &parsed
    }
    existing.DueDate = dueDate
}

// Agregar due_date al UPDATE SQL:
`UPDATE tasks SET title=$1, ..., due_date=$8, updated_at=NOW() WHERE id=$9`
```

---

## BAJO

### 22. Non-null assertion (`!`) abusado en Dashboard

| | |
|---|---|
| **Archivo** | `frontend/src/components/Dashboard.tsx:104-171` |
| **Problema** | El componente usa `stats!.` repetidamente despues de la validacion. TypeScript no puede inferir el narrowing a traves de la funcion `validateStats`. |
| **Impacto** | Fragilidad del tipo — si la logica de validacion cambia y deja pasar un `null`, se produce un runtime crash sin advertencia de TypeScript. |
| **Criticidad** | BAJA — Code smell que puede causar bugs futuros |
| **Tiempo de fix** | 10 minutos |
| **Valor** | Type safety completa, codigo mas robusto |

**Solucion recomendada:**
```typescript
// Usar type guard que TypeScript entienda:
function validateStats(stats: DashboardStats | null): stats is DashboardStats {
    if (!stats) return false;
    if (typeof stats.total_tasks !== 'number') return false;
    if (!stats.by_status || typeof stats.by_status !== 'object') return false;
    if (!stats.by_priority || typeof stats.by_priority !== 'object') return false;
    if (typeof stats.overdue_tasks !== 'number') return false;
    return true;
}

// Despues del guard, TypeScript sabe que stats es DashboardStats:
if (!validateStats(stats)) return <div>Invalid data</div>;
// stats.total_tasks   <-- sin ! necesario
```

---

### 23. useEffect con dependencia faltante

| | |
|---|---|
| **Archivo** | `frontend/src/app/tasks/[id]/page.tsx:30-34` |
| **Problema** | `loadTask` esta definida dentro del componente pero no esta en el dependency array del `useEffect`. Viola la regla `exhaustive-deps` de React. |
| **Impacto** | Funciona actualmente porque `loadTask` solo depende de `taskId`, pero si se agrega logica que dependa de otro estado, podria causar stale closures dificiles de debuggear. |
| **Criticidad** | BAJA — Riesgo futuro de bugs sutiles |
| **Tiempo de fix** | 5 minutos |
| **Valor** | Cumplimiento de reglas de React, prevencion de bugs |

**Solucion recomendada:**
```tsx
// Opcion 1: useCallback
const loadTask = useCallback(async () => {
    // ... logica
}, [taskId]);

useEffect(() => {
    if (taskId) loadTask();
}, [taskId, loadTask]);

// Opcion 2: Definir la funcion dentro del effect
useEffect(() => {
    if (!taskId) return;
    async function loadTask() {
        // ... logica
    }
    loadTask();
}, [taskId]);
```

---

### 24. CORS hardcoded a localhost

| | |
|---|---|
| **Archivo** | `backend/cmd/server/main.go:36-41` |
| **Problema** | `AllowedOrigins: []string{"http://localhost:3000"}` — solo funciona en desarrollo local. |
| **Impacto** | Cualquier despliegue fuera de localhost requiere cambiar el codigo fuente. No se puede configurar por ambiente. |
| **Criticidad** | BAJA — Bloquea despliegues sin cambio de codigo |
| **Tiempo de fix** | 10 minutos |
| **Valor** | Configuracion flexible por ambiente |

**Solucion recomendada:**
```go
allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
if allowedOrigins == "" {
    allowedOrigins = "http://localhost:3000"
}
origins := strings.Split(allowedOrigins, ",")

corsHandler := cors.New(cors.Options{
    AllowedOrigins: origins,
    // ... resto igual
})
```

---

### 25. Errores de json.Encode ignorados en responses

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:106, 169, 258, 365, 410, 454, 511` |
| **Problema** | `json.NewEncoder(w).Encode(tasks)` — el error retornado nunca se verifica. Si el cliente se desconecta o el buffer falla, el error se pierde. |
| **Impacto** | Bajo en la practica. El error mas comun (cliente desconectado) no es recuperable de todos modos. Pero violar la convencion Go de verificar errores dificulta la deteccion de problemas reales. |
| **Criticidad** | BAJA — Violacion de buenas practicas Go |
| **Tiempo de fix** | 15 minutos |
| **Valor** | Consistencia de codigo, mejor observabilidad |

**Solucion recomendada:**
```go
if err := json.NewEncoder(w).Encode(tasks); err != nil {
    log.Printf("error encoding response: %v", err)
    // No retornar http.Error aqui porque headers ya fueron enviados
}
```

---

### 26. Sin rate limiting en endpoint de login

| | |
|---|---|
| **Archivo** | `backend/cmd/server/main.go:51` |
| **Problema** | `/api/auth/login` no tiene rate limiting. Si se implementa validacion de password real, queda vulnerable a fuerza bruta. |
| **Impacto** | Actualmente mitigado porque cualquier password funciona (#4), pero al fixear ese bug, se abre inmediatamente a ataques de fuerza bruta. |
| **Criticidad** | BAJA — Riesgo latente (activado al fixear #4) |
| **Tiempo de fix** | 30 minutos |
| **Valor** | Proteccion proactiva contra brute force, defence-in-depth |

**Solucion recomendada:**
```go
// Usar middleware de rate limiting de chi:
import "github.com/go-chi/httprate"

r.Post("/api/auth/login",
    httprate.LimitByIP(5, time.Minute), // 5 intentos por minuto por IP
    handler.LoginHandler,
)
```

---

### 27. Sin proteccion CSRF

| | |
|---|---|
| **Archivo** | `backend/cmd/server/main.go` (completo) |
| **Problema** | CORS usa `AllowCredentials: true` pero no hay validacion de token CSRF. Actualmente mitigado porque se usa Bearer token en header (no cookies), pero si se migra a cookies (#11), se abre inmediatamente a CSRF. |
| **Impacto** | Riesgo latente. Con la arquitectura actual (Bearer token) no es explotable, pero al implementar la solucion de #11 (httpOnly cookies) seria inmediatamente vulnerable. |
| **Criticidad** | BAJA — Riesgo condicional/futuro |
| **Tiempo de fix** | 1 hora |
| **Valor** | Defence-in-depth, preparacion para migracion a cookies |

**Solucion recomendada:**
```go
// Implementar junto con la migracion a cookies:
// 1. Generar CSRF token y enviarlo en cookie no-httpOnly
// 2. Verificar que el header X-CSRF-Token coincida con la cookie
// 3. Usar middleware como github.com/gorilla/csrf
```

---

### 28. Sin limite de tamano en request body

| | |
|---|---|
| **Archivo** | `backend/internal/handler/tasks.go:181, 266` |
| **Problema** | `json.NewDecoder(r.Body).Decode(&req)` sin `http.MaxBytesReader`. Un atacante puede enviar payloads JSON de gigabytes, consumiendo toda la memoria del server. |
| **Impacto** | Vector de DoS: un solo request malicioso puede causar OOM (out of memory) y crashear el servidor. |
| **Criticidad** | BAJA — DoS requiere acceso autenticado |
| **Tiempo de fix** | 10 minutos |
| **Valor** | Proteccion contra abuso de recursos, estabilidad del servidor |

**Solucion recomendada:**
```go
// Opcion 1: En cada handler
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB max

// Opcion 2: Middleware global (recomendado)
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
            next.ServeHTTP(w, r)
        })
    }
}

// En main.go:
r.Use(MaxBodySize(1 << 20)) // 1MB global
```

---

## Matriz de Priorizacion

| # | Severidad | Esfuerzo | Valor | Prioridad de Fix |
|---|-----------|----------|-------|-------------------|
| 1 | CRITICO | 5 min | Maximo | **Inmediato** |
| 2 | CRITICO | 10 min | Maximo | **Inmediato** |
| 3 | CRITICO | 10 min | Maximo | **Inmediato** |
| 4 | CRITICO | 15 min | Alto | **Inmediato** |
| 5 | ALTO | 15 min | Alto | Sprint actual |
| 6 | ALTO | 20 min | Alto | Sprint actual |
| 9 | ALTO | 10 min | Alto | Sprint actual |
| 10 | ALTO | 15 min | Alto | Sprint actual |
| 8 | ALTO | 10 min | Medio | Sprint actual |
| 7 | ALTO | 1-2h | Alto | Sprint actual |
| 11 | ALTO | 1-2h | Alto | Proximo sprint |
| 12 | ALTO | 30 min | Medio | Proximo sprint |
| 13 | MEDIO | 1-2h | Alto | Proximo sprint |
| 14 | MEDIO | 30 min | Medio | Proximo sprint |
| 3 (update err) | MEDIO | 10 min | Medio | Proximo sprint |
| 16 | MEDIO | 1-2h | Alto | Backlog |
| 17 | MEDIO | 5 min | Medio | Sprint actual |
| 18 | MEDIO | 30 min | Alto | Proximo sprint |
| 19 | MEDIO | 5 min | Bajo | Backlog |
| 20 | MEDIO | 30 min | Medio | Backlog |
| 21 | MEDIO | 20 min | Medio | Backlog |
| 22-28 | BAJO | Variable | Bajo-Medio | Backlog |

---

## Recomendacion Final

**Accion inmediata (< 1 hora):** Fixear los 4 criticos (#1, #2, #3, #4). Son fixes de 5-15 minutos cada uno con impacto maximo en la confiabilidad del sistema.

**Sprint actual:** Consolidar JWT secret (#5, #6), proteger credenciales (#9, #10), y corregir busqueda (#8). Son fixes rapidos que cierran las vulnerabilidades de seguridad mas evidentes.

**Proximo sprint:** Abordar performance (N+1 queries #13, paginacion #16), storage seguro de tokens (#11), graceful shutdown (#18) y audit trail completo (#20).
