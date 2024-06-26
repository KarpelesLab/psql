package psql

import (
	"context"
	"fmt"
	"runtime"
)

// Logger is compatible with go's slog.Logger
type Logger interface {
	DebugContext(ctx context.Context, msg string, args ...any)
}

var logOutput Logger

// SetLogger sets a global logger for debugging psql
// This can be called easily as follows using go's slog package:
//
// psql.SetLogger(slog.Default())
//
// Or a better option:
//
// psql.SetLogger(slog.Default().With("backend", "psql")) etc...
func SetLogger(l Logger) {
	logOutput = l
}

func debugLog(ctx context.Context, msg string, args ...any) {
	if d := logOutput; d != nil {
		// do not add prefix here as it can be configured by the log package
		d.DebugContext(ctx, fmt.Sprintf(msg, args...), "event", "psql:debug")
	}
}

// debugStack returns a formatted stack trace of the goroutine that calls it.
// It calls runtime.Stack with a large enough buffer to capture the entire trace.
func debugStack() string {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}
