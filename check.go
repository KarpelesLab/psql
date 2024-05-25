package psql

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

func (t *TableMeta[T]) checkStructure(ctx context.Context) error {
	if v, ok := t.attrs["check"]; ok && v == "0" {
		// do not check table
		return nil
	}
	// SHOW FULL FIELDS FROM `table`
	// SHOW TABLE STATUS LIKE 'table' (engine)
	// SHOW INDEX FROM `table`
	// SELECT * FROM information_schema.TABLE_CONSTRAINTS WHERE `CONSTRAINT_SCHEMA` = '.$this->quote($this->database).' AND `TABLE_SCHEMA` = '.$this->quote($this->database).' AND `TABLE_NAME` = '.$this->quote($table_name).' AND `CONSTRAINT_TYPE` = \'FOREIGN KEY\'

	// The optional FULL keyword causes the output to include the column collation and comments, as well as the privileges you have for each column.
	res, err := t.backend.db.Query("SHOW FULL FIELDS FROM " + QuoteName(t.table))
	if err != nil {
		if IsNotExist(err) {
			// We simply need to create this table
			return t.createTable(ctx)
		}
		return err
	}
	defer res.Close()

	slog.Debug(fmt.Sprintf("[psql] Checking structure of table %s", t.table), "event", "psql:check", "psql.table", t.table)

	// index fields by name
	flds := make(map[string]*structField)
	for _, f := range t.fields {
		if _, found := flds[f.column]; found {
			return fmt.Errorf("invalid table structure, field %s.%s is defined multiple times", t.table, f.column)
		}
		flds[f.column] = f
	}

	var alterData []string

	var fInfo = &ShowFieldsResult{}
	for res.Next() {
		err = Table[ShowFieldsResult]().ScanTo(res, fInfo)
		if err != nil {
			return err
		}

		f, ok := flds[fInfo.Field]
		if !ok {
			slog.Warn(fmt.Sprintf("[psql:check] unused field %s.%s in structure", t.table, fInfo.Field), "event", "psql:check:unused_field", "psql.table", t.table, "psql.field", fInfo.Field)
			// TODO check if there is a DROP or RENAME rule for this field
			continue
		}
		delete(flds, fInfo.Field)
		ok, err := f.matches(fInfo.Type, fInfo.Null, fInfo.Collation, fInfo.Default)
		if err != nil {
			return fmt.Errorf("field %s.%s fails check: %w", t.table, fInfo.Field, err)
		}
		if !ok {
			// generate alter query
			alterData = append(alterData, "MODIFY "+f.defString())
		}
		// field=Log__ typ=char(36) col=latin1_general_ci null=NO key=PRI, dflt=%!s(*string=<nil>) xtra= priv=select,insert,update,references comment=
		// field=Secure_Key__ typ=char(36) col=latin1_general_ci null=NO key=, dflt=%!s(*string=0xc0000b6420) xtra= priv=select,insert,update,references comment=
		//log.Printf("field=%s typ=%s col=%s null=%s key=%s, dflt=%s xtra=%s priv=%s comment=%s", field, typ, col, null, key, dflt, xtra, priv, comment)
	}
	for _, f := range flds {
		alterData = append(alterData, "ADD "+f.defString())
	}

	res, err = t.backend.db.Query("SHOW INDEX FROM " + QuoteName(t.table))
	if err != nil {
		return fmt.Errorf("while doing SHOW INDEX: %w", err)
	}
	defer res.Close()

	// index keys by name
	keys := make(map[string]*structKey)
	for _, k := range t.keys {
		n := k.keyname()
		if _, found := keys[n]; found {
			return fmt.Errorf("invalid table structure, key %s.%s is defined multiple times", t.table, n)
		}
		keys[n] = k
	}

	var kInfo = &ShowIndexResult{}
	for res.Next() {
		err = Table[ShowIndexResult]().ScanTo(res, kInfo)
		if err != nil {
			return err
		}
		k, ok := keys[kInfo.KeyName]
		if !ok {
			slog.Warn(fmt.Sprintf("[psql:check] unused key %s.%s in structure", t.table, kInfo.KeyName), "event", "psql:check:unused_key", "psql.table", t.table, "psql.key", kInfo.KeyName)
			// TODO check if there is a DROP or RENAME rule for this key
			continue
		}
		delete(keys, kInfo.KeyName)
		ok, err := k.matches(kInfo)
		if err != nil {
			return fmt.Errorf("key %s.%s fails check: %w", t.table, kInfo.KeyName, err)
		}
		if !ok {
			// we can't change a key, but we can drop & recreate it
			alterData = append(alterData, "DROP "+k.sqlKeyName())
			alterData = append(alterData, "ADD "+k.defString())
		}
	}
	for _, k := range keys {
		alterData = append(alterData, "ADD "+k.defString())
	}

	// TODO: SHOW TABLE STATUS LIKE 'table'
	// → check Engine

	// TODO: SELECT * FROM information_schema.TABLE_CONSTRAINTS WHERE `CONSTRAINT_SCHEMA` = %database AND `TABLE_SCHEMA` = %database AND `TABLE_NAME` = %table AND `CONSTRAINT_TYPE` = 'FOREIGN KEY'
	// → check foreign keys

	if len(alterData) > 0 {
		s := &strings.Builder{}
		s.WriteString("ALTER TABLE ")
		s.WriteString(QuoteName(t.table))
		s.WriteByte(' ')
		for n, req := range alterData {
			if n > 0 {
				s.WriteString(", ")
			}
			s.WriteString(req)
		}
		slog.Debug(fmt.Sprintf("[psql] Performing: %s", s.String()), "event", "psql:check:perform_alter", "table", t.table)
		_, err := t.backend.db.Exec(s.String())
		if err != nil {
			return fmt.Errorf("while updating table %s: %w", t.table, err)
		}
	}
	return nil
}

func (t *TableMeta[T]) createTable(ctx context.Context) error {
	slog.DebugContext(ctx, fmt.Sprintf("[psql] Creating table %s", t.table), "event", "psql:check:create_table", "table", t.table)

	// Prepare a CREATE TABLE query
	s := &strings.Builder{}
	s.WriteString("CREATE TABLE ")
	s.WriteString(QuoteName(t.table))
	s.WriteString(" (")

	for n, field := range t.fields {
		if n > 0 {
			s.WriteString(", ")
		}
		s.WriteString(field.defString())
	}
	for _, key := range t.keys {
		s.WriteString(", ")
		s.WriteString(key.defString())
	}
	// TODO add keys
	s.WriteString(")")

	slog.DebugContext(ctx, fmt.Sprintf("[psql] Performing: %s", s.String()), "event", "psql:check:perform_create", "table", t.table)
	_, err := t.backend.db.Exec(s.String())
	if err != nil {
		return fmt.Errorf("while creating table %s: %w", t.table, err)
	}
	return nil
}
