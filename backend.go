package psql

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// Backend represents a database connection with engine-specific behavior.
// Create one with [New], or use a submodule constructor (e.g., mysql.New, pgsql.New,
// sqlite.New), then attach it to a context with [Backend.Plug] or [ContextBackend].
type Backend struct {
	db         *sql.DB
	driverData any // engine-specific data (e.g., *pgxpool.Pool)
	engine     Engine
	checked    map[reflect.Type]bool
	checkedLk  sync.RWMutex
	namer      Namer // custom namer for table/column names
}

// New returns a [Backend] that connects to the database identified by dsn.
// The engine is auto-detected by trying registered [BackendFactory] implementations.
// Import a database submodule (e.g., _ "github.com/portablesql/psql-sqlite") to
// register its factory.
func New(dsn string) (*Backend, error) {
	for _, f := range backendFactories {
		if f.MatchDSN(dsn) {
			return f.CreateBackend(dsn)
		}
	}
	return nil, fmt.Errorf("no backend factory matches DSN: %s (did you import a database driver submodule?)", dsn)
}

// NewBackend creates a Backend with the given engine and *sql.DB. This is called
// by submodule factories to construct backends.
func NewBackend(engine Engine, db *sql.DB, opts ...BackendOption) *Backend {
	b := &Backend{
		db:      db,
		engine:  engine,
		checked: make(map[reflect.Type]bool),
		namer:   &LegacyNamer{},
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// BackendOption is a functional option for [NewBackend].
type BackendOption func(*Backend)

// WithDriverData sets engine-specific driver data (e.g., *pgxpool.Pool for PostgreSQL).
func WithDriverData(data any) BackendOption {
	return func(b *Backend) {
		b.driverData = data
	}
}

// WithNamer sets the naming strategy for table/column names.
func WithNamer(n Namer) BackendOption {
	return func(b *Backend) {
		b.namer = n
	}
}

// WithPoolDefaults configures standard connection pool settings (128 max open,
// 32 max idle, 3 min lifetime).
func WithPoolDefaults(b *Backend) {
	b.db.SetConnMaxLifetime(time.Minute * 3)
	b.db.SetMaxOpenConns(128)
	b.db.SetMaxIdleConns(32)
}

// Plug attaches this backend to the given context. All psql operations using
// the returned context will use this backend. Equivalent to [ContextBackend].
func (be *Backend) Plug(ctx context.Context) context.Context {
	return ContextBackend(ctx, be)
}

// DB returns the underlying *sql.DB connection. Panics if the backend is nil.
func (be *Backend) DB() *sql.DB {
	if be == nil {
		panic("attempting to perform DB operations without backend")
	}
	return be.db
}

// Engine returns the database engine type ([EngineMySQL], [EnginePostgreSQL], or [EngineSQLite]).
func (be *Backend) Engine() Engine {
	if be == nil {
		return EngineUnknown
	}
	return be.engine
}

// DriverData returns the engine-specific driver data (e.g., *pgxpool.Pool for PostgreSQL).
func (be *Backend) DriverData() any {
	if be == nil {
		return nil
	}
	return be.driverData
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
