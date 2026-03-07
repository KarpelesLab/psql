package psql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type Backend struct {
	db        *sql.DB       // db backend, always set
	pgdb      *pgxpool.Pool // pgx backend, if any
	engine    Engine
	checked   map[reflect.Type]bool
	checkedLk sync.RWMutex
	namer     Namer // custom namer for table/column names
}

// New returns a Backend that connects to the provided database
func New(dsn string) (*Backend, error) {
	if strings.HasPrefix(dsn, "postgresql://") {
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return nil, err
		}
		return NewPG(cfg)
	}
	if strings.HasPrefix(dsn, "sqlite:") || strings.HasPrefix(dsn, "file:") || strings.HasSuffix(dsn, ".db") || strings.HasSuffix(dsn, ".sqlite") || strings.HasSuffix(dsn, ".sqlite3") || dsn == ":memory:" {
		return NewSQLite(strings.TrimPrefix(dsn, "sqlite:"))
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	return NewMySQL(cfg)
}

func NewMySQL(cfg *mysql.Config) (*Backend, error) {
	cfg.Params = map[string]string{
		"charset":  "utf8mb4",
		"sql_mode": "'ANSI,NO_BACKSLASH_ESCAPES'",
	}

	// use db to check
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(128)
	db.SetMaxIdleConns(32)

	res, err := db.Query("SHOW VARIABLES LIKE 'version%'")
	if err != nil {
		return nil, fmt.Errorf("SHOW VARIABLES failed: %w", err)
	}

	defer res.Close()
	for res.Next() {
		var k, v string
		if err := res.Scan(&k, &v); err != nil {
			panic(err)
		}
		slog.Debug(fmt.Sprintf("[mysql] %s = %s", k, v), "event", "psql:init:dbvar", "psql.dbvar", k)
	}

	b := &Backend{
		db:      db,
		engine:  EngineMySQL,
		checked: make(map[reflect.Type]bool),
		namer:   &LegacyNamer{}, // Default to LegacyNamer for backward compatibility
	}

	return b, nil
}

func NewPG(cfg *pgxpool.Config) (*Backend, error) {
	pgdb, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	b := &Backend{
		db:      stdlib.OpenDBFromPool(pgdb),
		pgdb:    pgdb,
		engine:  EnginePostgreSQL,
		checked: make(map[reflect.Type]bool),
		namer:   &LegacyNamer{}, // Default to LegacyNamer for backward compatibility
	}
	b.db.SetConnMaxLifetime(time.Minute * 3)
	b.db.SetMaxOpenConns(128)
	b.db.SetMaxIdleConns(32)

	return b, nil
}

func NewSQLite(dsn string) (*Backend, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite connection failed: %w", err)
	}

	// SQLite doesn't handle concurrent writes well, limit to 1 open connection
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	// Enable WAL mode and foreign keys
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		slog.Warn(fmt.Sprintf("[sqlite] failed to enable WAL mode: %s", err), "event", "psql:init:sqlite_wal")
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		slog.Warn(fmt.Sprintf("[sqlite] failed to enable foreign keys: %s", err), "event", "psql:init:sqlite_fk")
	}

	b := &Backend{
		db:      db,
		engine:  EngineSQLite,
		checked: make(map[reflect.Type]bool),
		namer:   &LegacyNamer{},
	}

	return b, nil
}

func (be *Backend) Plug(ctx context.Context) context.Context {
	return ContextBackend(ctx, be)
}

func (be *Backend) DB() *sql.DB {
	if be == nil {
		panic("attempting to perform DB operations without backend")
	}
	return be.db
}

func (be *Backend) Engine() Engine {
	if be == nil {
		return EngineUnknown
	}
	return be.engine
}

// Namer returns the configured naming strategy
func (be *Backend) Namer() Namer {
	if be == nil || be.namer == nil {
		// If backend or namer is nil, return LegacyNamer for backward compatibility
		return &LegacyNamer{}
	}
	return be.namer
}

// SetNamer allows changing the naming strategy
// Use DefaultNamer to keep names exactly as defined in structs (e.g., "HelloWorld" stays "HelloWorld")
// Use LegacyNamer (default) for backward compatibility (e.g., "HelloWorld" becomes "Hello_World")
// Use CamelSnakeNamer to convert all names to Camel_Snake_Case
func (be *Backend) SetNamer(n Namer) {
	if be == nil {
		return
	}
	be.namer = n
}

// checkOnce return true if a table has been checked once, or false otherwise
func (be *Backend) checkedOnce(typ reflect.Type) bool {
	if be.isChecked(typ) {
		return true
	}

	be.checkedLk.Lock()
	defer be.checkedLk.Unlock()

	// re-check now that we have an exclusive lock
	_, ok := be.checked[typ]
	if ok {
		return true
	}

	// set to true & return false
	be.checked[typ] = true
	return false
}

func (be *Backend) isChecked(typ reflect.Type) bool {
	be.checkedLk.RLock()
	defer be.checkedLk.RUnlock()
	_, ok := be.checked[typ]
	return ok
}
