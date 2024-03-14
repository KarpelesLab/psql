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
		&testV{psql.B().Select().From("Table"), `SELECT * FROM "Table"`},
		&testV{psql.B().Select("Field").From("Table"), `SELECT "Field" FROM "Table"`},
		&testV{psql.B().Select(psql.V("Value")).From("Table"), `SELECT 'Value' FROM "Table"`},
		&testV{psql.B().Select("Field").From("Table").Where(&psql.Like{psql.F("Field"), "prefix%"}), `SELECT "Field" FROM "Table" WHERE ("Field" LIKE 'prefix%' ESCAPE '\')`},
		&testV{psql.B().Select("Field").From("Table").Where(psql.Equal(psql.F("Field"), []byte{0xff, 0x00, 0xbe, 0xef})), `SELECT "Field" FROM "Table" WHERE ("Field"=x'ff00beef')`},
		&testV{psql.B().Select(psql.Raw("COUNT(1)")).From("Table"), `SELECT COUNT(1) FROM "Table"`},
		&testV{psql.B().Update("Table").Set(map[string]any{"Col": "Value", "Col2": "Value 2"}).Where(psql.Equal(psql.F("Field"), 42)), `UPDATE "Table" SET "Col"='Value',"Col2"='Value 2' WHERE ("Field"=42)`},
		&testV{psql.B().Select("Field").From("Table").OrderBy(psql.S("Col1", "ASC"), psql.S("Col2")), `SELECT "Field" FROM "Table" ORDER BY "Col1" ASC,"Col2"`},
		&testV{psql.B().Select(psql.Raw(`"A", "B"`)).From("Table"), `SELECT "A", "B" FROM "Table"`},
		&testV{psql.B().Select("Field").From("Table").Where(map[string]any{"Field": psql.WhereOR{nil, psql.Lte(nil, 42)}}), `SELECT "Field" FROM "Table" WHERE (("Field" IS NULL OR "Field" <= 42))`},
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

func TestBuilderArgs(t *testing.T) {
	// Various builds
	tests := []*testV{
		&testV{psql.B().Select().From("Table"), `SELECT * FROM "Table"`},
		&testV{psql.B().Select("Field").From("Table"), `SELECT "Field" FROM "Table"`},
		&testV{psql.B().Select(psql.V("Value")).From("Table"), `SELECT ? FROM "Table"`},
		&testV{psql.B().Select("Field").From("Table").Where(&psql.Like{psql.F("Field"), "prefix%"}), `SELECT "Field" FROM "Table" WHERE ("Field" LIKE ? ESCAPE '\')`},
		&testV{psql.B().Select("Field").From("Table").Where(psql.Equal(psql.F("Field"), []byte{0xff, 0x00, 0xbe, 0xef})), `SELECT "Field" FROM "Table" WHERE ("Field"=?)`},
		&testV{psql.B().Select(psql.Raw("COUNT(1)")).From("Table"), `SELECT COUNT(1) FROM "Table"`},
		&testV{psql.B().Update("Table").Set(map[string]any{"Col": "Value", "Col2": "Value 2"}).Where(psql.Equal(psql.F("Field"), 42)), `UPDATE "Table" SET "Col"=?,"Col2"=? WHERE ("Field"=?)`},
		&testV{psql.B().Select("Field").From("Table").OrderBy(psql.S("Col1", "ASC"), psql.S("Col2")), `SELECT "Field" FROM "Table" ORDER BY "Col1" ASC,"Col2"`},
		&testV{psql.B().Select(psql.Raw(`"A", "B"`)).From("Table"), `SELECT "A", "B" FROM "Table"`},
	}

	for _, test := range tests {
		v, _, err := test.b.RenderArgs()
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
