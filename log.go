package psql

import "context"

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
// psql.SetLogger(log.New(os.Stderr, "psql:", log.LstdFlags))
func SetLogger(l Logger) {
	logOutput = l
}

func debugLog(ctx context.Context, msg string, args ...any) {
	if d := logOutput; d != nil {
		// do not add prefix here as it can be configured by the log package
		d.Printf(msg, args...)
	}
}
