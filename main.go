package psql

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Init starts the database pool and allows all other methods in psql to work
func Init(dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return err
	}
	return InitCfg(cfg)
}

func InitCfg(cfg *mysql.Config) error {
	var err error
	cfg.Params = map[string]string{
		"charset":  "utf8mb4",
		"sql_mode": "'ANSI,NO_BACKSLASH_ESCAPES'",
	}

	// use db to check
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(128)
	db.SetMaxIdleConns(32)

	res, err := db.Query("SHOW VARIABLES LIKE 'version%'")
	if err != nil {
		return fmt.Errorf("SHOW VARIABLES failed: %w", err)
	}

	defer res.Close()
	for res.Next() {
		var k, v string
		if err := res.Scan(&k, &v); err != nil {
			panic(err)
		}
		slog.Debug(fmt.Sprintf("[mysql] %s = %s", k, v), "event", "psql:init:dbvar", "psql.dbvar", k)
	}

	return nil
}
