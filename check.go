package psql

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
)

// check will run checkStructure if it hasn't been run yet on this connection
func (t *TableMeta[T]) check(ctx context.Context) {
	be := GetBackend(ctx)
	if be.checkedOnce(t.typ) {
		return
	}

	// perform check
	switch be.engine {
	case EngineMySQL:
		err := t.checkStructureMySQL(ctx)
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("psql: failed to check table %s: %s", t.table, err), "event", "psql:table:check_error", "psql.table", t.table)
		}
	case EnginePostgreSQL:
		err := t.checkStructurePG(ctx)
		if err != nil {
			log.Printf("err = %s", err)
			slog.ErrorContext(ctx, fmt.Sprintf("psql: failed to check table %s: %s", t.table, err), "event", "psql:table:check_error", "psql.table", t.table)
		}
	}
}

func (t *TableMeta[T]) checkStructurePG(ctx context.Context) error {
	if v, ok := t.attrs["check"]; ok && v == "0" {
		// do not check table
		return nil
	}

	// table = &{Virtual:{st:0xc00003e500} Catalog:defaultdb Schema:public Table:Test_Table1 TableType:BASE TABLE}
	tinfo, err := QT[pgSchemaTables]("SELECT * FROM information_schema.tables WHERE table_catalog = current_database() AND table_schema = current_schema() AND table_name = $2", t.table).Single(ctx)
	if err != nil {
		if IsNotExist(err) {
			// We simply need to create this table
			return t.createTablePG(ctx)
		}
		return err
	}
	if tinfo.TableType != "BASE TABLE" {
		return fmt.Errorf("cannot check tables of type %s", tinfo.TableType)
	}

	cols, err := QT[pgSchemaColumns]("SELECT * FROM information_schema.columns WHERE table_catalog = current_database() AND table_schema = current_schema() AND table_name = $1", t.table).All(ctx)
	if err != nil {
		return err
	}

	// index fields by name
	flds := make(map[string]*structField)
	for _, f := range t.fields {
		if _, found := flds[f.column]; found {
			return fmt.Errorf("invalid table structure, field %s.%s is defined multiple times", t.table, f.column)
		}
		flds[f.column] = f
	}

	var alterData []string

	for _, fInfo := range cols {
		f, ok := flds[fInfo.Column]
		if !ok {
			slog.Warn(fmt.Sprintf("[psql:check] field %s.%s missing in structure", t.table, fInfo.Column), "event", "psql:check:unused_field", "psql.table", t.table, "psql.field", fInfo.Column)
			// TODO check if there is a DROP or RENAME rule for this field
			continue
		}
		delete(flds, fInfo.Column)
		ok, err := f.matches(EnginePostgreSQL, fInfo.DataType, string(fInfo.IsNullable), nil, nil) // fInfo.Collation, fInfo.Default)
		if err != nil {
			return fmt.Errorf("field %s.%s fails check: %w", t.table, fInfo.Column, err)
		}
		if !ok {
			// generate alter query
			//alterData = append(alterData, "MODIFY "+f.defString(EnginePostgreSQL))
			// TODO ALTER of fields is not GA on cockroach
		}
		// field=Log__ typ=char(36) col=latin1_general_ci null=NO key=PRI, dflt=%!s(*string=<nil>) xtra= priv=select,insert,update,references comment=
		// field=Secure_Key__ typ=char(36) col=latin1_general_ci null=NO key=, dflt=%!s(*string=0xc0000b6420) xtra= priv=select,insert,update,references comment=
		//log.Printf("field=%s typ=%s col=%s null=%s key=%s, dflt=%s xtra=%s priv=%s comment=%s", field, typ, col, null, key, dflt, xtra, priv, comment)
	}
	for _, f := range flds {
		alterData = append(alterData, "ADD "+f.defString(EnginePostgreSQL))
	}

	// index keys by name
	keys := make(map[string]*structKey)
	for _, k := range t.keys {
		n := k.keyname()
		if _, found := keys[n]; found {
			return fmt.Errorf("invalid table structure, key %s.%s is defined multiple times", t.table, n)
		}
		keys[n] = k
	}
	indices, err := QT[pgShowIndex]("SHOW INDEX FROM " + QuoteName(t.table)).All(ctx)
	if err != nil {
		return fmt.Errorf("while doing SHOW INDEX: %w", err)
	}

	for _, kInfo := range indices {
		k, ok := keys[kInfo.Index]
		if !ok {
			slog.Warn(fmt.Sprintf("[psql:check] key %s.%s missing in structure", t.table, kInfo.Index), "event", "psql:check:unused_key", "psql.table", t.table, "psql.key", kInfo.Index)
			// TODO check if there is a DROP or RENAME rule for this key
			continue
		}
		delete(keys, kInfo.Index)
		ok, err := k.matchesPG(kInfo)
		if err != nil {
			return fmt.Errorf("key %s.%s fails check: %w", t.table, kInfo.Index, err)
		}
		if !ok {
			// we can't change a key, but we can drop & recreate it
			alterData = append(alterData, "DROP "+k.sqlKeyName())
			alterData = append(alterData, "ADD "+k.defString(EnginePostgreSQL))
		}
	}
	for _, k := range keys {
		alterData = append(alterData, "ADD "+k.defString(EnginePostgreSQL))
	}

	// TODO: SHOW TABLE STATUS LIKE 'table'
	// → check Engine

	// TODO: SELECT * FROM information_schema.TABLE_CONSTRAINTS WHERE `CONSTRAINT_SCHEMA` = %database AND `TABLE_SCHEMA` = %database AND `TABLE_NAME` = %table AND `CONSTRAINT_TYPE` = 'FOREIGN KEY'
	// → check foreign keys

	// TODO
	// SET enable_experimental_alter_column_type_general = true; cockroach does not support modifying a column without that
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
		log.Printf("alter = %s", s)
		slog.Debug(fmt.Sprintf("[psql] Performing: %s", s.String()), "event", "psql:check:perform_alter", "table", t.table)
		err = Q(s.String()).Exec(ctx)
		if err != nil {
			return fmt.Errorf("while updating table %s: %w", t.table, err)
		}
	}
	return nil

	return nil
}

func (t *TableMeta[T]) checkStructureMySQL(ctx context.Context) error {
	if v, ok := t.attrs["check"]; ok && v == "0" {
		// do not check table
		return nil
	}
	// SHOW FULL FIELDS FROM `table`
	// SHOW TABLE STATUS LIKE 'table' (engine)
	// SHOW INDEX FROM `table`
	// SELECT * FROM information_schema.TABLE_CONSTRAINTS WHERE `CONSTRAINT_SCHEMA` = '.$this->quote($this->database).' AND `TABLE_SCHEMA` = '.$this->quote($this->database).' AND `TABLE_NAME` = '.$this->quote($table_name).' AND `CONSTRAINT_TYPE` = \'FOREIGN KEY\'

	// The optional FULL keyword causes the output to include the column collation and comments, as well as the privileges you have for each column.
	fList, err := QT[mysqlShowFieldsResult]("SHOW FULL FIELDS FROM " + QuoteName(t.table)).All(ctx)
	if err != nil {
		if IsNotExist(err) {
			// We simply need to create this table
			return t.createTableMySQL(ctx)
		}
		return err
	}

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

	for _, fInfo := range fList {
		f, ok := flds[fInfo.Field]
		if !ok {
			slog.Warn(fmt.Sprintf("[psql:check] unused field %s.%s in structure", t.table, fInfo.Field), "event", "psql:check:unused_field", "psql.table", t.table, "psql.field", fInfo.Field)
			// TODO check if there is a DROP or RENAME rule for this field
			continue
		}
		delete(flds, fInfo.Field)
		ok, err := f.matches(EngineMySQL, fInfo.Type, fInfo.Null, fInfo.Collation, fInfo.Default)
		if err != nil {
			return fmt.Errorf("field %s.%s fails check: %w", t.table, fInfo.Field, err)
		}
		if !ok {
			// generate alter query
			alterData = append(alterData, "MODIFY "+f.defString(EngineMySQL))
		}
		// field=Log__ typ=char(36) col=latin1_general_ci null=NO key=PRI, dflt=%!s(*string=<nil>) xtra= priv=select,insert,update,references comment=
		// field=Secure_Key__ typ=char(36) col=latin1_general_ci null=NO key=, dflt=%!s(*string=0xc0000b6420) xtra= priv=select,insert,update,references comment=
		//log.Printf("field=%s typ=%s col=%s null=%s key=%s, dflt=%s xtra=%s priv=%s comment=%s", field, typ, col, null, key, dflt, xtra, priv, comment)
	}
	for _, f := range flds {
		alterData = append(alterData, "ADD "+f.defString(EngineMySQL))
	}

	kList, err := QT[mysqlShowIndexResult]("SHOW INDEX FROM " + QuoteName(t.table)).All(ctx)
	if err != nil {
		return fmt.Errorf("while doing SHOW INDEX: %w", err)
	}

	// index keys by name
	keys := make(map[string]*structKey)
	for _, k := range t.keys {
		n := k.keyname()
		if _, found := keys[n]; found {
			return fmt.Errorf("invalid table structure, key %s.%s is defined multiple times", t.table, n)
		}
		keys[n] = k
	}

	for _, kInfo := range kList {
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
			alterData = append(alterData, "ADD "+k.defString(EngineMySQL))
		}
	}
	for _, k := range keys {
		alterData = append(alterData, "ADD "+k.defString(EngineMySQL))
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
		err = Q(s.String()).Exec(ctx)
		if err != nil {
			return fmt.Errorf("while updating table %s: %w", t.table, err)
		}
	}
	return nil
}

func (t *TableMeta[T]) createTablePG(ctx context.Context) error {
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
		s.WriteString(field.defString(EnginePostgreSQL))
	}
	for _, key := range t.keys {
		s.WriteString(", ")
		s.WriteString(key.defString(EnginePostgreSQL))
	}
	// TODO add keys
	s.WriteString(")")

	slog.DebugContext(ctx, fmt.Sprintf("[psql] Performing: %s", s.String()), "event", "psql:check:perform_create", "table", t.table)
	err := Q(s.String()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("while creating table %s: %w", t.table, err)
	}
	return nil
}

func (t *TableMeta[T]) createTableMySQL(ctx context.Context) error {
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
		s.WriteString(field.defString(EngineMySQL))
	}
	for _, key := range t.keys {
		s.WriteString(", ")
		s.WriteString(key.defString(EngineMySQL))
	}
	// TODO add keys
	s.WriteString(")")

	slog.DebugContext(ctx, fmt.Sprintf("[psql] Performing: %s", s.String()), "event", "psql:check:perform_create", "table", t.table)
	err := Q(s.String()).Exec(ctx)
	if err != nil {
		return fmt.Errorf("while creating table %s: %w", t.table, err)
	}
	return nil
}
