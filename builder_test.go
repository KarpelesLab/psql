package psql_test

import (
	"testing"

	"github.com/KarpelesLab/psql"
)

type testV struct {
	b *psql.QueryBuilder
	v string
}

func TestBuilder(t *testing.T) {
	// Various builds
	tests := []*testV{
		&testV{psql.Q().Select().From("Table"), `SELECT * FROM "Table"`},
		&testV{psql.Q().Select("Field").From("Table"), `SELECT "Field" FROM "Table"`},
		&testV{psql.Q().Select(psql.V("Value")).From("Table"), `SELECT 'Value' FROM "Table"`},
		&testV{psql.Q().Select("Field").From("Table").Where(&psql.Like{psql.FieldName("Field"), "prefix%"}), `SELECT "Field" FROM "Table" WHERE ("Field" LIKE 'prefix%' ESCAPE '\')`},
		&testV{psql.Q().Select("Field").From("Table").Where(psql.Equal(psql.FieldName("Field"), []byte{0xff, 0x00, 0xbe, 0xef})), `SELECT "Field" FROM "Table" WHERE ("Field"=x'ff00beef')`},
		&testV{psql.Q().Select(psql.Raw("COUNT(1)")).From("Table"), `SELECT COUNT(1) FROM "Table"`},
		&testV{psql.Q().Update("Table").Set(&psql.Set{"Col", "Value"}, &psql.Set{"Col2", "Value 2"}).Where(psql.Equal(psql.FieldName("Field"), 42)), `UPDATE "Table" SET "Col"='Value',"Col2"='Value 2' WHERE ("Field"=42)`},
	}

	for _, test := range tests {
		v, err := test.b.Render()
		if err != nil {
			t.Errorf("Failed to render: %s", err)
			continue
		}

		if v != test.v {
			t.Errorf("bad render, got %s but expected %s", v, test.v)
			continue
		}
	}
}
