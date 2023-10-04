package psql

import (
	"context"
	"log"
	"runtime"
)

// Logger is compatible with go's log.Logger
type Logger interface {
	Printf(format string, v ...any)
}

var logOutput Logger

// SetLogger sets a global logger for debugging psql
// This can be called easily as follows using go's log package:
//
// psql.SetLogger(log.Default())
//
// Or a better option:
//
// psql.SetLogger(log.New(os.Stderr, "psql: ", log.LstdFlags|log.Lmsgprefix))
func SetLogger(l Logger) {
	logOutput = l
}

func debugLog(ctx context.Context, msg string, args ...any) {
	if d := logOutput; d != nil {
		// do not add prefix here as it can be configured by the log package
		d.Printf(msg, args...)
	}
}

// fatal error logging
func errorLog(ctx context.Context, msg string, args ...any) {
	if d := logOutput; d != nil {
		d.Printf(msg, args...)
	} else {
		log.Printf(msg, args...)
		log.Printf("[sql] Runtime stack:\n%s", debugStack())
	}
}

// debugStack returns a formatted stack trace of the goroutine that calls it.
// It calls runtime.Stack with a large enough buffer to capture the entire trace.
func debugStack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}
