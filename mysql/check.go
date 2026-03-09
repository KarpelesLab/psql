package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/KarpelesLab/psql"
)

func checkStructureMySQL(ctx context.Context, be *psql.Backend, tv psql.TableView) error {
	tableName := tv.FormattedName(be)

	if v, ok := tv.TableAttrs()["check"]; ok && v == "0" {
		return nil
	}

	sb := &strings.Builder{}
	sb.WriteString("SHOW TABLES LIKE '")
	sb.WriteString(strings.ReplaceAll(tableName, "'", "\\'"))
	sb.WriteString("'")

	var found bool
	err := psql.Q(sb.String()).Each(ctx, func(rows *sql.Rows) error {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		found = true
		return nil
	})
	if err != nil {
		return err
	}

	if !found {
		return createTableMySQL(ctx, be, tv)
	}

	// SHOW FIELDS for table
	sb.Reset()
	sb.WriteString("SHOW FIELDS FROM ")
	sb.WriteString(psql.QuoteName(tableName))

	type fieldInfo struct {
		Field, Type, Null, Key, Extra string
		Default                       *string
	}

	var fieldInfos []fieldInfo
	err = psql.Q(sb.String()).Each(ctx, func(rows *sql.Rows) error {
		cols, _ := rows.Columns()
		var fi fieldInfo
		var priv, comment string
		vars := []any{&fi.Field, &fi.Type, &fi.Null, &fi.Key, &fi.Default, &fi.Extra}
		if len(cols) >= 8 {
			vars = append(vars, &priv, &comment)
		} else if len(cols) >= 7 {
			vars = append(vars, &priv)
		}
		if err := rows.Scan(vars...); err != nil {
			return err
		}
		fieldInfos = append(fieldInfos, fi)
		return nil
	})
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

	for _, fi := range fieldInfos {
		f, ok := flds[fi.Field]
		if !ok {
			slog.Warn(fmt.Sprintf("[psql:check] field %s.%s missing in structure", tv.TableName(), fi.Field), "event", "psql:check:unused_field", "psql.table", tv.TableName(), "psql.field", fi.Field)
			continue
		}
		delete(flds, fi.Field)
		ok, err := f.Matches(be, fi.Type, fi.Null, &fi.Key, fi.Default)
		if err != nil {
			return fmt.Errorf("field %s.%s fails check: %w", tv.TableName(), fi.Field, err)
		}
		if !ok {
			alterData = append(alterData, "MODIFY "+f.DefString(be))
		}
	}
	for _, f := range flds {
		alterData = append(alterData, "ADD "+f.DefString(be))
	}

	// index keys by name
	keys := make(map[string]*psql.StructKey)
	for _, k := range tv.AllKeys() {
		if k.Index >= 0 {
			keys[k.Key] = k
		} else {
			keys[k.Name] = k
		}
	}

	sb.Reset()
	sb.WriteString("SHOW INDEX FROM ")
	sb.WriteString(psql.QuoteName(tableName))

	type keyInfo struct {
		name     string
		nonuniq  bool
		keytype  string
		keyparts map[string]string
	}
	keydata := make(map[string]*keyInfo)

	err = psql.Q(sb.String()).Each(ctx, func(rows *sql.Rows) error {
		cols, _ := rows.Columns()
		var nTable, nNonUnique, nKey, nSeq *string
		var nCol, nCollation, nCardinality, nSub, nPacked, nNull, nType, nComment *string
		var nIndexComment, nVisible, nExpr *string

		vars := []any{&nTable, &nNonUnique, &nKey, &nSeq, &nCol, &nCollation, &nCardinality, &nSub, &nPacked, &nNull, &nType, &nComment}
		if len(cols) >= 15 {
			vars = append(vars, &nIndexComment, &nVisible, &nExpr)
		} else if len(cols) >= 14 {
			vars = append(vars, &nIndexComment, &nVisible)
		} else if len(cols) >= 13 {
			vars = append(vars, &nIndexComment)
		}

		if err := rows.Scan(vars...); err != nil {
			return err
		}

		ki, ok := keydata[*nKey]
		if !ok {
			ki = &keyInfo{
				name:     *nKey,
				nonuniq:  *nNonUnique == "1",
				keytype:  *nType,
				keyparts: make(map[string]string),
			}
			keydata[*nKey] = ki
		}
		if *nCol != "" {
			ki.keyparts[*nCol] = *nSeq
		}
		return nil
	})
	if err != nil {
		return err
	}

	for keyname := range keydata {
		if _, ok := keys[keyname]; ok {
			delete(keys, keyname)
			continue
		}
		slog.Warn(fmt.Sprintf("[psql:check] key %s.%s missing in structure", tv.TableName(), keyname), "event", "psql:check:unused_key", "psql.table", tv.TableName(), "psql.key", keyname)
	}
	for _, k := range keys {
		alterData = append(alterData, "ADD "+k.DefString(be))
	}

	if len(alterData) > 0 {
		sb.Reset()
		sb.WriteString("ALTER TABLE ")
		sb.WriteString(psql.QuoteName(tableName))
		sb.WriteByte(' ')
		for n, req := range alterData {
			if n > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(req)
		}
		slog.Debug(fmt.Sprintf("[psql] Performing: %s", sb.String()), "event", "psql:check:perform_alter", "table", tv.TableName())
		err = psql.Q(sb.String()).Exec(ctx)
		if err != nil {
			return fmt.Errorf("while updating table %s: %w", tv.TableName(), err)
		}
	}
	return nil
}

func createTableMySQL(ctx context.Context, be *psql.Backend, tv psql.TableView) error {
	tableName := tv.FormattedName(be)

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

	for _, k := range tv.AllKeys() {
		if len(k.Fields) == 0 {
			continue
		}
		sb.WriteString(", ")
		sb.WriteString(k.DefString(be))
	}

	sb.WriteByte(')')

	if err := psql.Q(sb.String()).Exec(ctx); err != nil {
		return fmt.Errorf("while creating structure: %w", err)
	}

	return nil
}
