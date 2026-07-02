# Logging

The Midaz Go SDK is silent by default. Configure a `*slog.Logger` via
`midaz.WithLogger(...)` to receive structured log lines for retry attempts,
slow API calls, and internal warnings.

## v3 contract

- `*slog.Logger` is the canonical logging contract. No bespoke `Logger`
  interface, no `Fatal/Fatalf`, no `MIDAZ_DEBUG`-driven stderr bypass.
- `c.Logger()` always returns a non-nil `*slog.Logger`. The default is a
  discard handler — silent unless you opt in.
- The SDK never calls `os.Exit` on behalf of the host. Fatal conditions
  surface as returned errors.
- `MIDAZ_DEBUG=true` set via `config.FromEnvironment()` installs a stderr
  text handler at debug level **only when no `WithLogger` was supplied**.
  An explicit `WithLogger` always wins.

## Quickstart

```go
import (
    "context"
    "log/slog"
    "os"
    "time"

    midaz "github.com/LerianStudio/midaz-sdk-golang/v4"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    c, err := midaz.New(
        midaz.WithLogger(logger),
        midaz.WithAnonymous(),
        midaz.WithSlowCallThreshold(2 * time.Second),
    )
    if err != nil { return }
    defer c.Shutdown(context.Background())

    // ...c.Accounts.Get(...) etc. Retry attempts emit DEBUG/WARN lines
    // through `logger`. Calls slower than 2s emit a WARN line.
}
```

## What gets logged

| Event | Level | Component | Notes |
|-------|-------|-----------|-------|
| Retry attempt (intermediate) | DEBUG | `retry` | `cause`, `delay_ms`, `attempt`, `max_retries` |
| Retry attempt (final before exhaustion) | WARN | `retry` | Same fields, but at WARN so it surfaces in production filters |
| Retry exhausted | WARN | `http` | `attempts`, `max_retries`, `error`, `http.status_code`, `request_id` when available |
| Slow API call | WARN | `http` | `duration_ms`, `threshold_ms`, `http.status_code`, `request_id` |
| HTTP request phase failed | WARN | `http` | Build, validation, marshal, send, read, or decode failures with `phase` and `error.category` when available |
| HTTP response error | WARN | `http` | Terminal non-2xx response when it is not a retry-exhausted error |
| Token refresh started or succeeded | DEBUG | `http` | Reactive Access Manager token refetch after a 401 response |
| Token refresh failed | WARN | `http` | Failed reactive Access Manager token refetch |
| Access Manager insecure HTTP opt-in | WARN | `bootstrap` | Emitted at construction when insecure Access Manager HTTP is explicitly allowed |
| Debug-mode request/response (when `WithDebug(true)`) | DEBUG | `http` | Sanitized URL + body |

SDK diagnostic log lines include:

- `sdk.name = "midaz-go-sdk"`
- `sdk.component` — emitting subsystem (`retry`, `http`, or `bootstrap`)
- `operation` — HTTP method or service operation

Request-related log lines also include `http.method` and a normalized `url.path` where the SDK can derive a request URL.

## Integrations

### stdlib `slog` (Go 1.21+)

```go
import (
    "log/slog"
    "os"
)

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
c, _ := midaz.New(midaz.WithLogger(logger), midaz.WithAnonymous())
```

For pretty text output instead of JSON:

```go
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
    AddSource: true,
}))
```

### charmbracelet/log

`charmbracelet/log` implements `slog.Handler` directly:

```go
import (
    "log/slog"
    "os"

    charm "github.com/charmbracelet/log"
)

clog := charm.NewWithOptions(os.Stderr, charm.Options{
    ReportCaller:    true,
    ReportTimestamp: true,
    Level:           charm.DebugLevel,
})

c, _ := midaz.New(midaz.WithLogger(slog.New(clog)), midaz.WithAnonymous())
```

### Uber zap (via slog adapter)

```go
import (
    "log/slog"

    "go.uber.org/zap"
    "go.uber.org/zap/exp/zapslog"
)

zl, _ := zap.NewProduction()
defer zl.Sync()

c, _ := midaz.New(
    midaz.WithLogger(slog.New(zapslog.NewHandler(zl.Core(), nil))),
    midaz.WithAnonymous(),
)
```

### rs/zerolog (via slog adapter)

```go
import (
    "log/slog"
    "os"

    "github.com/rs/zerolog"
    slogzerolog "github.com/samber/slog-zerolog/v2"
)

zlog := zerolog.New(os.Stderr).With().Timestamp().Logger()
handler := slogzerolog.Option{Level: slog.LevelDebug, Logger: &zlog}.NewZerologHandler()

c, _ := midaz.New(
    midaz.WithLogger(slog.New(handler)),
    midaz.WithAnonymous(),
)
```

## Slow-call warnings

`midaz.WithSlowCallThreshold(d)` emits a `WARN`-level log line whenever a
successful API call exceeds `d`. The line includes `duration_ms`,
`threshold_ms`, `http.status_code`, and `request_id`.

```go
c, _ := midaz.New(
    midaz.WithLogger(logger),
    midaz.WithAnonymous(),
    midaz.WithSlowCallThreshold(2 * time.Second),
)
```

Setting the threshold without a logger is harmless — the warning lands on
the discard handler.

Zero or negative values disable the warning entirely.

## MIDAZ_DEBUG and FromEnvironment

When the caller opts into env-driven config and sets `MIDAZ_DEBUG=true`:

```go
cfg, _ := config.NewConfig(config.FromEnvironment())
c, _ := midaz.New(midaz.WithConfig(cfg)) // no WithLogger
```

…the SDK installs a default `slog.Logger` at debug level writing to stderr
in text format. This is the only path that produces console output without
explicit logger configuration.

If you also call `WithLogger(...)`, your logger wins and `MIDAZ_DEBUG` is
ignored as a logger source (the `Config.Debug` field still toggles the
SDK's verbose request/response logs through your logger at debug level).

## Trace correlation

The SDK's `*slog.Logger` surface does not add `trace_id` and `span_id` by itself. Add those fields by using a slog handler that enriches records from the context, or attach them manually with `observability.TraceID(ctx)` and `observability.SpanID(ctx)`. The separate `observability.Provider.Logger()` surface can also attach span IDs through `Logger.WithSpan(span)`.

For manual enrichment:

```go
import "log/slog"

traceID := observability.TraceID(ctx)
spanID := observability.SpanID(ctx)

c.Logger().LogAttrs(ctx, slog.LevelInfo, "processing request",
    slog.String("trace_id", traceID),
    slog.String("span_id",  spanID),
    slog.String("tenant_id", "acme-corp"),
)
```

## What was removed in v3

- `MIDAZ_DEBUG=true` setting in shell no longer installs a logger
  unless the caller routes through `config.FromEnvironment()`.
- `Logger.Fatal` and `Logger.Fatalf` deleted from the public interface.
  Library code must not terminate the host process.
- `MIDAZ_DEBUG` bypass at `entities/http.go` (raw text to stderr) is gone.
  Debug-mode logs go through the configured `*slog.Logger` like everything else.
- 3 unconditional `fmt.Fprintf(os.Stderr, ...)` calls were removed:
  pagination misuse warnings (relocated to Track 5's typed `ListOpts`),
  HTTP optimizer best-effort errors (now silent fallback), and an
  unaligned-offset warning (relocated to typed list opts validation through
  `ValidatePageListOpts`, `ValidateCursorListOpts`, and each entity's
  `XxxListOpts.Validate()`).
