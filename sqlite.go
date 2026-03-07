package psql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

func (t *TableMeta[T]) checkStructureSQLite(ctx context.Context, be *Backend) error {
	if v, ok := t.attrs["check"]; ok && v == "0" {
		return nil
	}

	tableName := t.FormattedName(be)

	// Check if table exists
	var count int
	err := Q("SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?", tableName).Each(ctx, func(rows *sql.Rows) error {
		return rows.Scan(&count)
	})
	if err != nil {
		return fmt.Errorf("while checking table existence: %w", err)
	}

	if count == 0 {
		return t.createTableSQLite(ctx, be)
	}

	// Table exists, check columns via PRAGMA table_info
	type pragmaCol struct {
		CID        int
		Name       string
		Type       string
		NotNull    int
		DefaultVal *string
		PK         int
	}

	var existingCols []pragmaCol
	err = Q(fmt.Sprintf("PRAGMA table_info(%s)", QuoteName(tableName))).Each(ctx, func(rows *sql.Rows) error {
		var c pragmaCol
		if err := rows.Scan(&c.CID, &c.Name, &c.Type, &c.NotNull, &c.DefaultVal, &c.PK); err != nil {
			return err
		}
		existingCols = append(existingCols, c)
		return nil
	})
	if err != nil {
		return fmt.Errorf("while reading table info: %w", err)
	}

	// Index existing columns by name
	colSet := make(map[string]bool)
	for _, c := range existingCols {
		colSet[c.Name] = true
	}

	// Check for missing columns
	for _, f := range t.fields {
		if colSet[f.column] {
			continue
		}
		// Add missing column
		colDef := f.defStringSQLiteAlter(be)
		if colDef == "" {
			continue
		}
		alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", QuoteName(tableName), colDef)
		slog.Debug(fmt.Sprintf("[psql] SQLite ALTER: %s", alterSQL), "event", "psql:check:alter_sqlite", "table", t.table)
		if err := Q(alterSQL).Exec(ctx); err != nil {
			return fmt.Errorf("while adding column to %s: %w", t.table, err)
		}
	}

	// Check for missing indexes
	existingIdxs := make(map[string]bool)
	err = Q(fmt.Sprintf("PRAGMA index_list(%s)", QuoteName(tableName))).Each(ctx, func(rows *sql.Rows) error {
		var seq int
		var name, origin string
		var unique, partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return err
		}
		existingIdxs[name] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("while reading index list: %w", err)
	}

	for _, k := range t.keys {
		if k.typ == keyPrimary {
			continue // PRIMARY KEY is part of table definition
		}
		idxName := tableName + "_" + k.key
		if existingIdxs[idxName] {
			continue
		}
		createSQL := k.createIndexSQLite(tableName)
		if createSQL == "" {
			continue
		}
		slog.Debug(fmt.Sprintf("[psql] Creating SQLite index: %s", createSQL), "event", "psql:check:create_index_sqlite", "table", t.table)
		if err := Q(createSQL).Exec(ctx); err != nil {
			return fmt.Errorf("while creating index on %s: %w", t.table, err)
		}
	}

	return nil
}

func (t *TableMeta[T]) createTableSQLite(ctx context.Context, be *Backend) error {
	tableName := t.FormattedName(be)

	sb := &strings.Builder{}
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(QuoteName(tableName))
	sb.WriteString(" (")

	// Fields
	for n, f := range t.fields {
		if n > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.defString(be))
	}

	// Inline constraints (PRIMARY KEY, UNIQUE)
	for _, k := range t.keys {
		if len(k.fields) == 0 {
			continue
		}
		if k.typ == keyPrimary {
			sb.WriteString(", PRIMARY KEY (")
			for i, f := range k.fields {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(QuoteName(f))
			}
			sb.WriteByte(')')
		} else if k.typ == keyUnique {
			sb.WriteString(", UNIQUE (")
			for i, f := range k.fields {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(QuoteName(f))
			}
			sb.WriteByte(')')
		}
	}

	sb.WriteByte(')')

	if err := Q(sb.String()).Exec(ctx); err != nil {
		return fmt.Errorf("while creating table: %w", err)
	}

	// Create non-inline indexes
	for _, k := range t.keys {
		if len(k.fields) == 0 || k.typ == keyPrimary || k.typ == keyUnique {
			continue
		}
		createSQL := k.createIndexSQLite(tableName)
		if createSQL == "" {
			continue
		}
		if err := Q(createSQL).Exec(ctx); err != nil {
			return fmt.Errorf("while creating index: %w", err)
		}
	}

	return nil
}

// defStringSQLiteAlter generates a column definition suitable for ALTER TABLE ADD COLUMN.
// SQLite cannot add a NOT NULL column without a default value when the table has existing rows.
func (f *structField) defStringSQLiteAlter(be *Backend) string {
	attrs := f.getAttrs(be)
	mytyp := f.sqlType(be)
	if mytyp == "" {
		return ""
	}

	mydef := QuoteName(f.column) + " " + mytyp

	hasDefault := false
	isNotNull := false

	if null, ok := attrs["null"]; ok {
		switch null {
		case "0", "false":
			isNotNull = true
		}
	}

	if def, ok := attrs["default"]; ok {
		hasDefault = true
		if def == "\\N" {
			mydef += " DEFAULT NULL"
		} else {
			mydef += " DEFAULT " + Escape(def)
		}
	}

	// SQLite requires a DEFAULT for NOT NULL columns added via ALTER TABLE
	if isNotNull {
		if !hasDefault {
			// Provide a sensible zero-value default based on type affinity
			switch mytyp {
			case "integer":
				mydef += " NOT NULL DEFAULT 0"
			case "real":
				mydef += " NOT NULL DEFAULT 0.0"
			default:
				mydef += " NOT NULL DEFAULT ''"
			}
		} else {
			mydef += " NOT NULL"
		}
	}

	return mydef
}

// createIndexSQLite generates a CREATE INDEX statement for SQLite.
func (k *structKey) createIndexSQLite(tableName string) string {
	s := &strings.Builder{}

	switch k.typ {
	case keyPrimary:
		return "" // handled inline
	case keyUnique:
		s.WriteString("CREATE UNIQUE INDEX ")
	case keyIndex:
		s.WriteString("CREATE INDEX ")
	default:
		// FULLTEXT, SPATIAL, VECTOR not supported in SQLite
		return ""
	}

	s.WriteString(QuoteName(tableName + "_" + k.key))
	s.WriteString(" ON ")
	s.WriteString(QuoteName(tableName))
	s.WriteString(" (")
	for n, f := range k.fields {
		if n > 0 {
			s.WriteString(", ")
		}
		s.WriteString(QuoteName(f))
	}
	s.WriteByte(')')
	return s.String()
}
