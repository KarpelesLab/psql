package psql

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

func (t *TableMeta) checkStructure() error {
	// SHOW FULL FIELDS FROM `table`
	// SHOW TABLE STATUS LIKE 'table' (engine)
	// SHOW INDEX FROM `table`
	// SELECT * FROM information_schema.TABLE_CONSTRAINTS WHERE `CONSTRAINT_SCHEMA` = '.$this->quote($this->database).' AND `TABLE_SCHEMA` = '.$this->quote($this->database).' AND `TABLE_NAME` = '.$this->quote($table_name).' AND `CONSTRAINT_TYPE` = \'FOREIGN KEY\'

	// The optional FULL keyword causes the output to include the column collation and comments, as well as the privileges you have for each column.
	res, err := db.Query(fmt.Sprintf("SHOW FULL FIELDS FROM `%s`", t.table))
	if err != nil {
		if IsNotExist(err) {
			// We simply need to create this table
			return t.createTable()
		}
		return err
	}
	defer res.Close()

	log.Printf("[psql] Checking structure of table %s", t.table)

	// index fields by name
	flds := make(map[string]*structField)
	for _, f := range t.fields {
		if _, found := flds[f.column]; found {
			return fmt.Errorf("invalid table structure, field %s.%s is defined multiple times", t.table, f.column)
		}
		flds[f.column] = f
	}

	for res.Next() {
		var field, typ, null, key, xtra, priv, comment string
		var dflt, col *string
		if err := res.Scan(&field, &typ, &col, &null, &key, &dflt, &xtra, &priv, &comment); err != nil {
			return err
		}

		f, ok := flds[field]
		if !ok {
			log.Printf("[psql:check] unused field %s.%s in structure", t.table, field)
			// TODO check if there is a DROP or RENAME rule for this field
			continue
		}
		ok, err := f.matches(typ, null, col, dflt)
		if err != nil {
			return fmt.Errorf("field %s.%s fails check: %w", t.table, field, err)
		}
		if !ok {
			// generate alter query
			log.Printf("[psql] NEED: ALTER TABLE `%s` CHANGE %s", t.table, f.defString())
		}
		// field=Log__ typ=char(36) col=latin1_general_ci null=NO key=PRI, dflt=%!s(*string=<nil>) xtra= priv=select,insert,update,references comment=
		// field=Secure_Key__ typ=char(36) col=latin1_general_ci null=NO key=, dflt=%!s(*string=0xc0000b6420) xtra= priv=select,insert,update,references comment=
		//log.Printf("field=%s typ=%s col=%s null=%s key=%s, dflt=%s xtra=%s priv=%s comment=%s", field, typ, col, null, key, dflt, xtra, priv, comment)
	}
	return nil
}

func (t *TableMeta) createTable() error {
	log.Printf("[psql] Creating table %s", t.table)

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
	// TODO add keys
	s.WriteString(")")

	_, err := db.Exec(s.String())
	if err != nil {
		return fmt.Errorf("while creating table %s: %w", t.table, err)
	}
	return nil
}

func (f *structField) matches(typ, null string, col, dflt *string) (bool, error) {
	if f.attrs == nil {
		return false, errors.New("no valid field defined")
	}

	if f.sqlType() != typ {
		return false, nil
	}

	if mycol, ok := f.attrs["collation"]; ok && col != nil && mycol != *col {
		// bad collation → alter
		return false, nil
	}

	return true, nil
}
