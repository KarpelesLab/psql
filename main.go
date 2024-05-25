package psql

import (
	"github.com/go-sql-driver/mysql"
)

var (
	DefaultBackend *Backend
)

// Init creates a new Backend and sets DefaultBackend
func Init(dsn string) error {
	be, err := New(dsn)
	if err != nil {
		return err
	}
	DefaultBackend = be
	return nil
}

// InitCfg creates a new MySQL Backend and sets DefaultBackend
func InitCfg(cfg *mysql.Config) error {
	be, err := NewMySQL(cfg)
	if err != nil {
		return err
	}
	DefaultBackend = be
	return nil
}
