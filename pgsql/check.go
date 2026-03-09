package pgsql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"strings"

	"github.com/KarpelesLab/psql"
)

func checkStructurePG(ctx context.Context, be *psql.Backend, tv psql.TableView) error {
	if v, ok := tv.TableAttrs()["check"]; ok && v == "0" {
		return nil
	}

	tableName := tv.FormattedName(be)

	tinfo, err := psql.QT[pgSchemaTables]("SELECT * FROM information_schema.tables WHERE table_catalog = current_database() AND table_schema = current_schema() AND table_name = $1", tableName).Single(ctx)
	if err != nil {
		if psql.IsNotExist(err) {
			return createTablePG(ctx, be, tv)
		}
		return err
	}
	if tinfo.TableType != "BASE TABLE" {
		return fmt.Errorf("cannot check tables of type %s", tinfo.TableType)
	}

	// Collect all enum constraints for this table
	constraints := psql.CollectEnumConstraints(tv, be)

	cols, err := psql.QT[pgSchemaColumns]("SELECT * FROM information_schema.columns WHERE table_catalog = current_database() AND table_schema = current_schema() AND table_name = $1", tableName).All(ctx)
	if err != nil {
		return err
	}

	// index fields by name
	flds := make(map[string]*psql.StructField)
	for _, f := range tv.AllFields() {
		if _, found := flds[f.Column]; found {
			return fmt.Errorf("invalid table structure, field %s.%s is defined multiple times", tv.TableName(), f.Column)
		}
		flds[f.Column] = f
	}

	var alterData []string

	for _, fInfo := range cols {
		f, ok := flds[fInfo.Column]
		if !ok {
			slog.Warn(fmt.Sprintf("[psql:check] field %s.%s missing in structure", tv.TableName(), fInfo.Column), "event", "psql:check:unused_field", "psql.table", tv.TableName(), "psql.field", fInfo.Column)
			continue
		}
		delete(flds, fInfo.Column)
		ok, err := f.Matches(be, fInfo.DataType, string(fInfo.IsNullable), nil, nil)
		if err != nil {
			return fmt.Errorf("field %s.%s fails check: %w", tv.TableName(), fInfo.Column, err)
		}
		if !ok {
			// TODO ALTER of fields is not GA on cockroach
		}
	}
	for _, f := range flds {
		alterData = append(alterData, "ADD "+f.DefString(be))
	}

	if len(alterData) > 0 {
		s := &strings.Builder{}
		s.WriteString("ALTER TABLE ")
		s.WriteString(psql.QuoteName(tableName))
		s.WriteByte(' ')
		for n, req := range alterData {
			if n > 0 {
				s.WriteString(", ")
			}
			s.WriteString(req)
		}
		log.Printf("alter = %s", s)
		slog.Debug(fmt.Sprintf("[psql] Performing: %s", s.String()), "event", "psql:check:perform_alter", "table", tv.TableName())
		err = psql.Q(s.String()).Exec(ctx)
		if err != nil {
			return fmt.Errorf("while updating table %s: %w", tv.TableName(), err)
		}

		alterData = nil
	}

	// index keys by name
	keys := make(map[string]*psql.StructKey)
	for _, k := range tv.AllKeys() {
		n := k.Keyname()
		if _, found := keys[n]; found {
			return fmt.Errorf("invalid table structure, key %s.%s is defined multiple times", tv.TableName(), n)
		}
		keys[n] = k
	}

	// Query existing indexes
	existingKeys := make(map[string]bool)
	err = psql.Q(`SELECT indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename = $1`, tableName).Each(ctx, func(rows *sql.Rows) error {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return err
		}
		existingKeys[indexName] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("while querying pg_indexes: %w", err)
	}

	for keyName := range existingKeys {
		if _, ok := keys[keyName]; ok {
			delete(keys, keyName)
			continue
		}
		if keyName == tableName+"_pkey" {
			delete(keys, "PRIMARY")
			continue
		}
		if strings.HasPrefix(keyName, tableName+"_") {
			rawName := strings.TrimPrefix(keyName, tableName+"_")
			if _, ok := keys[rawName]; ok {
				delete(keys, rawName)
				continue
			}
		}
		slog.Warn(fmt.Sprintf("[psql:check] key %s.%s missing in structure", tv.TableName(), keyName), "event", "psql:check:unused_key", "psql.table", tv.TableName(), "psql.key", keyName)
	}

	for _, k := range keys {
		createSQL := pgCreateIndex(k, tableName)
		if createSQL != "" {
			slog.Debug(fmt.Sprintf("[psql] Creating index: %s", createSQL), "event", "psql:check:create_index", "table", tv.TableName())
			if err := psql.Q(createSQL).Exec(ctx); err != nil {
				return fmt.Errorf("while creating index on table %s: %w", tv.TableName(), err)
			}
		}
	}

	// Add any missing CHECK constraints for enums
	for _, constraint := range constraints {
		var exists bool
		err := psql.Q(`SELECT EXISTS (
			SELECT 1 FROM pg_constraint c
			JOIN pg_class t ON c.conrelid = t.oid
			JOIN pg_namespace n ON t.relnamespace = n.oid
			WHERE c.conname = $1
			AND t.relname = $2
			AND n.nspname = current_schema()
		)`, constraint.Name, tableName).Each(ctx, func(rows *sql.Rows) error {
			return rows.Scan(&exists)
		})
		if err != nil {
			return fmt.Errorf("failed to check constraint %s: %w", constraint.Name, err)
		}

		if !exists {
			checkSQL := psql.GenerateEnumCheckSQL(constraint, tableName)
			if checkSQL != "" {
				alterSQL := fmt.Sprintf("ALTER TABLE %s ADD %s", psql.QuoteName(tableName), checkSQL)
				slog.Debug(fmt.Sprintf("[psql] Adding CHECK constraint: %s", alterSQL),
					"event", "psql:check:add_constraint",
					"table", tv.TableName(),
					"constraint", constraint.Name)
				if err := psql.Q(alterSQL).Exec(ctx); err != nil {
					return fmt.Errorf("failed to add CHECK constraint %s: %w", constraint.Name, err)
				}
			}
		}
	}

	return nil
}

func createTablePG(ctx context.Context, be *psql.Backend, tv psql.TableView) error {
	tableName := tv.FormattedName(be)

	constraints := psql.CollectEnumConstraints(tv, be)

	sb := &strings.Builder{}
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(psql.QuoteName(tableName))
	sb.WriteString(" (")

	for n, f := range tv.AllFields() {
		if n > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.DefString(be))
	}

	// Inline constraints (PRIMARY KEY and UNIQUE only)
	for _, k := range tv.AllKeys() {
		if len(k.Fields) == 0 {
			continue
		}
		if k.Typ == psql.KeyPrimary || k.Typ == psql.KeyUnique {
			sb.WriteString(", ")
			sb.WriteString(pgInlineKeyDef(k, tableName))
		}
	}

	// Add CHECK constraints for enums
	for _, constraint := range constraints {
		checkSQL := psql.GenerateEnumCheckSQL(constraint, tableName)
		if checkSQL != "" {
			sb.WriteString(", ")
			sb.WriteString(checkSQL)
		}
	}

	sb.WriteByte(')')

	if err := psql.Q(sb.String()).Exec(ctx); err != nil {
		return fmt.Errorf("while creating structure: %w", err)
	}

	// Create non-inline indexes
	for _, k := range tv.AllKeys() {
		if len(k.Fields) == 0 {
			continue
		}
		if k.Typ != psql.KeyPrimary && k.Typ != psql.KeyUnique {
			createSQL := pgCreateIndex(k, tableName)
			if createSQL != "" {
				if err := psql.Q(createSQL).Exec(ctx); err != nil {
					return fmt.Errorf("while creating index: %w", err)
				}
			}
		}
	}

	return nil
}
