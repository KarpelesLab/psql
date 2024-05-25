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
)

type Engine int

const (
	EngineUnknown Engine = iota
	EngineMySQL
	EnginePostgreSQL
)

type Backend struct {
	db     *sql.DB       // db backend, always set
	pgdb   *pgxpool.Pool // pgx backend, if any
	engine Engine

	tableMap  map[reflect.Type]TableMetaIntf
	tableMapL sync.RWMutex
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

	b := &Backend{db: db, engine: EngineMySQL, tableMap: make(map[reflect.Type]TableMetaIntf)}

	return b, nil
}

func NewPG(cfg *pgxpool.Config) (*Backend, error) {
	pgdb, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	b := &Backend{db: stdlib.OpenDBFromPool(pgdb), pgdb: pgdb, engine: EnginePostgreSQL, tableMap: make(map[reflect.Type]TableMetaIntf)}
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
