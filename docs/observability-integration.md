# GALA Platform - Observability & Error Handling

## Resumen de Cambios

Se han implementado los siguientes paquetes modulares para logging, manejo de errores y graceful shutdown:

```
backend/internal/pkg/
├── logger/           # Logging estructurado con slog
│   ├── logger.go
│   └── logger_test.go
├── errors/           # Errores con contexto y stack traces
│   ├── errors.go
│   └── errors_test.go
├── middleware/       # Middlewares HTTP
│   ├── middleware.go
│   └── middleware_test.go
└── shutdown/         # Graceful shutdown
    ├── shutdown.go
    └── shutdown_test.go
```

---

## 1. Logger (`pkg/logger`)

### Características
- Usa `slog` de Go 1.21+ (stdlib, sin dependencias externas)
- Output JSON estructurado por defecto
- Soporte para request ID y job ID en contexto
- Niveles: debug, info, warn, error

### Uso Básico

```go
import "gala/internal/pkg/logger"

// Crear logger
log := logger.New(logger.Config{
    Level:       "info",
    Format:      "json",
    ServiceName: "gala-api",
})

// Logging simple
log.Info("server started", "port", 8080)

// Con contexto
log.WithRequestID("req-123").Info("processing request")
log.WithJobID("job-456").Info("processing job")

// Desde contexto HTTP
ctx := logger.ContextWithRequestID(ctx, "req-123")
log.FromContext(ctx).Info("enriched log")

// Log de errores con stack trace
log.LogError(ctx, "operation failed", err, "user_id", "123")
```

### Variables de Entorno
- `LOG_LEVEL`: debug, info, warn, error (default: info)
- `LOG_FORMAT`: json, text (default: json)
- `LOG_SOURCE`: true/false - incluir archivo:línea (default: false)
- `SERVICE_NAME`: nombre del servicio

---

## 2. Errors (`pkg/errors`)

### Características
- Errores con códigos semánticos
- Stack traces automáticos
- Campos adicionales de contexto
- Mapeo automático a HTTP status codes

### Uso Básico

```go
import "gala/internal/pkg/errors"

// Crear error nuevo
err := errors.New(errors.CodeValidation, "email inválido")

// Con formato
err := errors.Newf(errors.CodeNotFound, "usuario %s no encontrado", userID)

// Constructores de conveniencia
err := errors.NotFound("user", "123")
err := errors.Validation("campo requerido")
err := errors.ValidationField("email", "formato inválido")
err := errors.Internal("error inesperado")
err := errors.Timeout("database query")

// Wrapping de errores
err := errors.Wrap(dbErr, "user.create", "failed to create user")
err := errors.WrapWithCode(dbErr, errors.CodeUnavailable, "db.query", "database unavailable")

// Agregar contexto
err := errors.NotFound("user", "123").
    WithField("email", "test@example.com").
    WithFields(map[string]any{"tenant": "acme"})

// Extraer información
code := errors.GetCode(err)         // CodeNotFound
status := errors.GetHTTPStatus(err) // 404
fields := errors.GetFields(err)     // map[string]any

// Checks
if errors.IsNotFound(err) { ... }
if errors.IsValidation(err) { ... }
if errors.IsConflict(err) { ... }
```

### Códigos Disponibles
| Code | HTTP Status |
|------|-------------|
| `CodeValidation` | 400 |
| `CodeBadRequest` | 400 |
| `CodeUnauthorized` | 401 |
| `CodeForbidden` | 403 |
| `CodeNotFound` | 404 |
| `CodeConflict` | 409 |
| `CodeAlreadyExists` | 409 |
| `CodeFailedPrecond` | 412 |
| `CodeResourceExhaust` | 429 |
| `CodeInternal` | 500 |
| `CodeUnavailable` | 503 |
| `CodeTimeout` | 504 |

---

## 3. Middleware (`pkg/middleware`)

### Características
- Request ID automático
- Logging de requests
- Recovery de panics
- Timeout de requests

### Uso en Router

```go
import (
    "github.com/go-chi/chi/v5"
    "gala/internal/pkg/middleware"
)

r := chi.NewRouter()

// Orden recomendado
r.Use(middleware.RequestID)           // Primero: genera/preserva request ID
r.Use(middleware.Recovery(log))       // Segundo: captura panics
r.Use(middleware.Logging(log))        // Tercero: logging de requests
r.Use(middleware.Timeout(30*time.Second)) // Opcional: timeout

// Handler con manejo de errores
r.Get("/users/{id}", middleware.WrapHandler(log, func(w http.ResponseWriter, r *http.Request) error {
    user, err := userService.Get(r.Context(), chi.URLParam(r, "id"))
    if err != nil {
        return err // El middleware convierte esto en respuesta JSON
    }
    return writeJSON(w, user)
}))
```

### Headers
- `X-Request-ID`: Se genera automáticamente si no existe, se preserva si ya viene en el request

---

## 4. Shutdown (`pkg/shutdown`)

### Características
- Manejo de señales SIGINT, SIGTERM, SIGHUP
- Timeout configurable
- Ejecución de handlers en paralelo
- Context cancelable

### Uso Básico

```go
import "gala/internal/pkg/shutdown"

// Crear manager
shutdownMgr := shutdown.NewManager(log, 30*time.Second)

// Registrar handlers (se ejecutan en orden inverso - LIFO)
shutdownMgr.Register("http-server", func(ctx context.Context) error {
    return server.Shutdown(ctx)
})
shutdownMgr.Register("postgres", func(ctx context.Context) error {
    pool.Close()
    return nil
})
shutdownMgr.RegisterSimple("redis", func() {
    rdb.Close()
})

// Esperar señal de shutdown (bloquea)
shutdownMgr.Wait()
```

---

## Integración con el Proyecto

### Archivos Modificados

1. **`cmd/api/main.go`**
   - Usa logger estructurado
   - Integra shutdown manager
   - Verifica conexiones al inicio

2. **`cmd/worker/main.go`**
   - Usa logger estructurado
   - Integra shutdown manager
   - Context cancelable para el worker loop

3. **`internal/httpapi/router.go`**
   - Integra middlewares de logging, recovery, request ID
   - Recibe logger en Deps

4. **`internal/httpapi/handlers/handlers.go`**
   - Recibe logger en Deps
   - Métodos helper para logging con contexto

5. **`internal/httpapi/handlers/health.go`**
   - Health check profundo (`?deep=true`)
   - Verifica PostgreSQL, Redis, Storage

6. **`internal/worker/run.go`**
   - Logging estructurado por job
   - Context con job ID

7. **`internal/worker/processor/processor.go`**
   - Usa `pkg/errors` para errores con contexto
   - Logging detallado de cada paso

---

## Ejecutar Tests

```bash
cd backend

# Todos los tests de pkg
go test ./internal/pkg/...

# Tests con verbose
go test -v ./internal/pkg/...

# Tests con coverage
go test -cover ./internal/pkg/...

# Test específico
go test -v ./internal/pkg/logger/
go test -v ./internal/pkg/errors/
go test -v ./internal/pkg/middleware/
go test -v ./internal/pkg/shutdown/
```

---

## Output de Logs (Ejemplo)

```json
{"time":"2025-01-15T10:30:00.000Z","level":"INFO","msg":"starting GALA API","service":"gala-api","version":"0.1.0"}
{"time":"2025-01-15T10:30:00.100Z","level":"INFO","msg":"PostgreSQL connected","service":"gala-api"}
{"time":"2025-01-15T10:30:00.150Z","level":"INFO","msg":"HTTP server listening","service":"gala-api","addr":"0.0.0.0:8080","port":"8080"}
{"time":"2025-01-15T10:30:05.000Z","level":"INFO","msg":"request completed","service":"gala-api","request_id":"abc123","method":"POST","path":"/jobs","status":201,"duration_ms":45}
{"time":"2025-01-15T10:30:10.000Z","level":"INFO","msg":"processing job","service":"gala-worker","component":"worker","job_id":"job_123"}
{"time":"2025-01-15T10:30:15.000Z","level":"ERROR","msg":"job failed","service":"gala-worker","job_id":"job_123","code":"VALIDATION_ERROR","op":"processor.parse","message":"invalid params"}
```

---

## Próximos Pasos

1. **Tests de integración**: Agregar tests que verifiquen el flujo completo API → Worker → Renderer
2. **Métricas**: Agregar Prometheus metrics (counters, histograms)
3. **Tracing**: Integrar OpenTelemetry para distributed tracing
4. **Retry logic**: Implementar reintentos en el worker para errores transitorios
