package psql

import (
	"github.com/go-sql-driver/mysql"
)

// DefaultBackend is the global backend used when no backend is attached to the context.
// Set it with [Init] or [InitCfg], or assign directly.
var DefaultBackend *Backend

// Init creates a new [Backend] using the given DSN and sets it as [DefaultBackend].
// See [New] for supported DSN formats.
func Init(dsn string) error {
	be, err := New(dsn)
	if err != nil {
		return err
	}
	DefaultBackend = be
	return nil
}

// InitCfg creates a new MySQL [Backend] from the given config and sets it as [DefaultBackend].
func InitCfg(cfg *mysql.Config) error {
	be, err := NewMySQL(cfg)
	if err != nil {
		return err
	}
	DefaultBackend = be
	return nil
}
