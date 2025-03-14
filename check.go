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
		err := t.checkStructureMySQL(ctx, be)
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("psql: failed to check table %s: %s", t.table, err), "event", "psql:table:check_error", "psql.table", t.table)
		}
	case EnginePostgreSQL:
		err := t.checkStructurePG(ctx, be)
		if err != nil {
			log.Printf("err = %s", err)
			slog.ErrorContext(ctx, fmt.Sprintf("psql: failed to check table %s: %s", t.table, err), "event", "psql:table:check_error", "psql.table", t.table)
		}
	}
}

func (t *TableMeta[T]) checkStructurePG(ctx context.Context, be *Backend) error {
	if v, ok := t.attrs["check"]; ok && v == "0" {
		// do not check table
		return nil
	}

	// table = &{Virtual:{st:0xc00003e500} Catalog:defaultdb Schema:public Table:Test_Table1 TableType:BASE TABLE}
	tinfo, err := QT[pgSchemaTables]("SELECT * FROM information_schema.tables WHERE table_catalog = current_database() AND table_schema = current_schema() AND table_name = $1", t.table).Single(ctx)
	if err != nil {
		if IsNotExist(err) {
			// We simply need to create this table
			return t.createTablePG(ctx, be)
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
		ok, err := f.matches(be, fInfo.DataType, string(fInfo.IsNullable), nil, nil) // fInfo.Collation, fInfo.Default)
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
		alterData = append(alterData, "ADD "+f.defString(be))
	}

	// run alter table now, keys do not work the same as fields with pgsql
	if len(alterData) > 0 {
		// TODO
		// SET enable_experimental_alter_column_type_general = true; cockroach does not support modifying a column without that
		be := GetBackend(ctx)

		// Format the table name using the namer
		tableName := be.Namer().TableName(t.table)

		s := &strings.Builder{}
		s.WriteString("ALTER TABLE ")
		s.WriteString(QuoteName(tableName))
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

		alterData = nil
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

	// Format the table name using the namer
	tableName := be.Namer().TableName(t.table)

	indices, err := QT[pgShowIndex]("SHOW INDEX FROM " + QuoteName(tableName)).All(ctx)
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
			alterData = append(alterData, "ADD "+k.defString(be))
		}
	}
	for _, k := range keys {
		// need to create this key
		alterData = append(alterData, "ADD "+k.defString(be))
	}

	// alter for keys
	if len(alterData) > 0 {
		s := &strings.Builder{}
		s.WriteString("ALTER TABLE ")
		s.WriteString(QuoteName(tableName))
		s.WriteByte(' ')
		for n, req := range alterData {
			if n > 0 {
				s.WriteString(", ")
			}
			s.WriteString(req)
		}
		slog.Warn(fmt.Sprintf("[psql] Would update keys: %s", s.String()), "event", "psql:check:would_alter", "table", t.table)
		//err = Q(s.String()).Exec(ctx)
		if err != nil {
			return fmt.Errorf("while updating key on table %s: %w", t.table, err)
		}

		alterData = nil
	}

	return nil
}

func (t *TableMeta[T]) createTablePG(ctx context.Context, be *Backend) error {
	// Format the table name using the namer
	tableName := be.Namer().TableName(t.table)

	// CREATE TABLE
	sb := &strings.Builder{}

	// build query
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(QuoteName(tableName))
	sb.WriteString(" (")

	// fields
	for n, f := range t.fields {
		if n > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.defString(be))
	}

	// keys & indexes
	for _, k := range t.keys {
		if len(k.fields) == 0 {
			continue
		}
		sb.WriteString(", ")
		sb.WriteString(k.defString(be))
	}

	// end query
	sb.WriteByte(')')

	if err := Q(sb.String()).Exec(ctx); err != nil {
		return fmt.Errorf("while creating structure: %w", err)
	}

	return nil
}

func (t *TableMeta[T]) checkStructureMySQL(ctx context.Context, be *Backend) error {
	// Format the table name using the namer
	tableName := be.Namer().TableName(t.table)

	// select TABLE_NAME from INFORMATION_SCHEMA.TABLES where TABLE_SCHEMA="database" and TABLE_NAME="table"
	if v, ok := t.attrs["check"]; ok && v == "0" {
		// do not check table
		return nil
	}

	sb := &strings.Builder{}

	// build query
	sb.WriteString("SHOW TABLES LIKE '")
	sb.WriteString(strings.ReplaceAll(tableName, "'", "\\'"))
	sb.WriteString("'")

	rows, err := doQueryContext(ctx, sb.String())
	if err != nil {
		return err
	}
	defer rows.Close()

	if !rows.Next() {
		// No matching table
		return t.createTableMySQL(ctx)
	}
	// SHOW FIELDS for table
	sb.Reset()
	sb.WriteString("SHOW FIELDS FROM ")
	sb.WriteString(QuoteName(tableName))

	rows, err = doQueryContext(ctx, sb.String())
	if err != nil {
		return err
	}
	defer rows.Close()

	// get all data from mysql - Field, Type, Null, Key, Default, Extra
	var field, typ, null, key, xtra, priv, comment string
	var dflt *string
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	var alterData []string

	// prepare scan for row
	vars := []any{&field, &typ, &null, &key, &dflt, &xtra}
	if len(cols) >= 8 {
		vars = append(vars, &priv, &comment)
	} else if len(cols) >= 7 {
		vars = append(vars, &priv)
	}

	// index fields by name
	flds := make(map[string]*structField)
	for _, f := range t.fields {
		if _, found := flds[f.column]; found {
			return fmt.Errorf("invalid table structure, field %s.%s is defined multiple times", t.table, f.column)
		}
		flds[f.column] = f
	}

	// scan each row
	for rows.Next() {
		err := rows.Scan(vars...)
		if err != nil {
			return err
		}

		// find column
		f, ok := flds[field]
		if !ok {
			slog.Warn(fmt.Sprintf("[psql:check] field %s.%s missing in structure", t.table, field), "event", "psql:check:unused_field", "psql.table", t.table, "psql.field", field)
			// TODO check if there is a DROP or RENAME rule for this field
			continue
		}
		delete(flds, field)
		ok, err = f.matches(be, typ, null, &key, dflt)
		if err != nil {
			return fmt.Errorf("field %s.%s fails check: %w", t.table, field, err)
		}
		if !ok {
			// generate alter query
			alterData = append(alterData, "MODIFY "+f.defString(be))
		}
		// field=Log__ typ=char(36) col=latin1_general_ci null=NO key=PRI, dflt=%!s(*string=<nil>) xtra= priv=select,insert,update,references comment=
		// field=Secure_Key__ typ=char(36) col=latin1_general_ci null=NO key=, dflt=%!s(*string=0xc0000b6420) xtra= priv=select,insert,update,references comment=
		//log.Printf("field=%s typ=%s col=%s null=%s key=%s, dflt=%s xtra=%s priv=%s comment=%s", field, typ, col, null, key, dflt, xtra, priv, comment)
	}
	for _, f := range flds {
		alterData = append(alterData, "ADD "+f.defString(be))
	}

	// index keys by name
	keys := make(map[string]*structKey)
	for _, k := range t.keys {
		if k.index >= 0 {
			keys[k.key] = k
		} else {
			keys[k.name] = k
		}
	}

	sb.Reset()
	sb.WriteString("SHOW INDEX FROM ")
	sb.WriteString(QuoteName(tableName))

	rows2, err := doQueryContext(ctx, sb.String())
	if err != nil {
		return err
	}
	defer rows2.Close()

	var keydata = make(map[string]*keyinfo)

	var nTable, nNonUnique, nKey, nSeq *string
	var nCol, nCollation, nCardinality, nSub, nPacked, nNull, nType, nComment, nExpr *string

	cols, err = rows2.Columns()
	if err != nil {
		return err
	}

	vars = []any{&nTable, &nNonUnique, &nKey, &nSeq, &nCol, &nCollation, &nCardinality, &nSub, &nPacked, &nNull, &nType, &nComment}
	if len(cols) >= 13 {
		vars = append(vars, &nExpr)
	}

	for rows2.Next() {
		err := rows2.Scan(vars...)
		if err != nil {
			return err
		}

		ki, ok := keydata[*nKey]
		if !ok {
			ki = &keyinfo{
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
	}

	// check key for table
	for keyname, keyinfo := range keydata {
		k, ok := keys[keyname]
		if !ok {
			slog.Warn(fmt.Sprintf("[psql:check] key %s.%s missing in structure", t.table, keyname), "event", "psql:check:unused_key", "psql.table", t.table, "psql.key", keyname)
			continue
		}
		delete(keys, keyname)
		ok, err := k.matches(keyinfo)
		if err != nil {
			return fmt.Errorf("key %s.%s fails check: %w", t.table, keyname, err)
		}
		if !ok {
			// we can't change a key, but we can drop & recreate it
			alterData = append(alterData, "DROP "+k.sqlKeyName())
			alterData = append(alterData, "ADD "+k.defString(be))
		}
	}
	for _, k := range keys {
		// need to create this key
		alterData = append(alterData, "ADD "+k.defString(be))
	}

	if len(alterData) > 0 {
		// run alter
		sb.Reset()
		sb.WriteString("ALTER TABLE ")
		sb.WriteString(QuoteName(tableName))
		sb.WriteByte(' ')
		for n, req := range alterData {
			if n > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(req)
		}
		slog.Debug(fmt.Sprintf("[psql] Performing: %s", sb.String()), "event", "psql:check:perform_alter", "table", t.table)
		err = Q(sb.String()).Exec(ctx)
		if err != nil {
			return fmt.Errorf("while updating table %s: %w", t.table, err)
		}
	}
	return nil
}

func (t *TableMeta[T]) createTableMySQL(ctx context.Context) error {
	be := GetBackend(ctx)
	// Format the table name using the namer
	tableName := be.Namer().TableName(t.table)

	// CREATE TABLE
	sb := &strings.Builder{}

	// build query
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(QuoteName(tableName))
	sb.WriteString(" (")

	// fields
	for n, f := range t.fields {
		if n > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.defString(be))
	}

	// keys & indexes
	for _, k := range t.keys {
		if len(k.fields) == 0 {
			continue
		}
		sb.WriteString(", ")
		sb.WriteString(k.defString(be))
	}

	// end query
	sb.WriteByte(')')

	if err := Q(sb.String()).Exec(ctx); err != nil {
		return fmt.Errorf("while creating structure: %w", err)
	}

	return nil
}

type keyinfo struct {
	name     string
	nonuniq  bool
	keytype  string
	keyparts map[string]string
}
