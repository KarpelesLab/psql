package psql_test

import (
	"testing"

	"github.com/portablesql/psql"
	"github.com/stretchr/testify/assert"
)

func TestFormatTableName(t *testing.T) {
	assert.Equal(t, "Hello_World", psql.FormatTableName("HelloWorld"))
	assert.Equal(t, "My_Table", psql.FormatTableName("MyTable"))
	assert.Equal(t, "Simple", psql.FormatTableName("Simple"))
	assert.Equal(t, "A_B_C", psql.FormatTableName("ABC"))
	assert.Equal(t, "Table1", psql.FormatTableName("Table1"))
}

func TestDefaultNamer(t *testing.T) {
	n := &psql.DefaultNamer{}

	assert.Equal(t, "MyTable", n.TableName("MyTable"))
	assert.Equal(t, "MySchema", n.SchemaName("MySchema"))
	assert.Equal(t, "col", n.ColumnName("tbl", "col"))
	assert.Equal(t, "JoinTbl", n.JoinTableName("JoinTbl"))
	assert.Equal(t, "chk_tbl_col", n.CheckerName("tbl", "col"))
	assert.Equal(t, "idx_tbl_col", n.IndexName("tbl", "col"))
	assert.Equal(t, "uniq_tbl_col", n.UniqueName("tbl", "col"))
	assert.Equal(t, "enum_tbl_col", n.EnumTypeName("tbl", "col"))
}

func TestCamelSnakeNamer(t *testing.T) {
	n := &psql.CamelSnakeNamer{}

	assert.Equal(t, "Hello_World", n.TableName("HelloWorld"))
	assert.Equal(t, "Hello_World", n.SchemaName("HelloWorld"))
	assert.Equal(t, "My_Col", n.ColumnName("tbl", "MyCol"))
	assert.Equal(t, "My_Join", n.JoinTableName("MyJoin"))
	assert.Equal(t, "chk_My_Table_My_Col", n.CheckerName("MyTable", "MyCol"))
	assert.Equal(t, "idx_My_Table_My_Col", n.IndexName("MyTable", "MyCol"))
	assert.Equal(t, "uniq_My_Table_My_Col", n.UniqueName("MyTable", "MyCol"))
	assert.Equal(t, "enum_My_Table_My_Col", n.EnumTypeName("MyTable", "MyCol"))
}

func TestLegacyNamer(t *testing.T) {
	n := &psql.LegacyNamer{}

	assert.Equal(t, "Hello_World", n.TableName("HelloWorld"))
	assert.Equal(t, "Hello_World", n.SchemaName("HelloWorld"))
	assert.Equal(t, "MyCol", n.ColumnName("tbl", "MyCol")) // Columns unchanged
	assert.Equal(t, "My_Join", n.JoinTableName("MyJoin"))
	assert.Equal(t, "chk_My_Table_MyCol", n.CheckerName("MyTable", "MyCol"))
	assert.Equal(t, "idx_My_Table_MyCol", n.IndexName("MyTable", "MyCol"))
	assert.Equal(t, "uniq_My_Table_MyCol", n.UniqueName("MyTable", "MyCol"))
	assert.Equal(t, "enum_My_Table_MyCol", n.EnumTypeName("MyTable", "MyCol"))
}

func TestBackendSetNamer(t *testing.T) {
	be := &psql.Backend{}

	// Default namer should be LegacyNamer
	n := be.Namer()
	assert.IsType(t, &psql.LegacyNamer{}, n)

	// Set to DefaultNamer
	be.SetNamer(&psql.DefaultNamer{})
	n = be.Namer()
	assert.IsType(t, &psql.DefaultNamer{}, n)

	// Set to CamelSnakeNamer
	be.SetNamer(&psql.CamelSnakeNamer{})
	n = be.Namer()
	assert.IsType(t, &psql.CamelSnakeNamer{}, n)

	// SetNamer on nil should not panic
	var nilBe *psql.Backend
	nilBe.SetNamer(&psql.DefaultNamer{})
}
