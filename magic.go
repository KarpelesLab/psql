package psql

import "fmt"

var magicEngineTypes = map[Engine]map[string]string{
	EngineMySQL: map[string]string{
		"DATETIME": "type=DATETIME,size=6",
	},
	EnginePostgreSQL: map[string]string{
		"DATETIME": "type=TIMESTAMP,size=6,default='1970-01-01 00:00:00.000000'",
	},
}

// quickhand types that can be imported easily, for example via `sql:",import=UUID"`
var magicTypes = map[string]string{
	"UUID":     "type=CHAR,size=36,default=00000000-0000-0000-0000-000000000000,collation=latin1_general_ci,validator=uuid",
	"INT":      "type=INT,size=11",
	"BIGINT":   "type=BIGINT,size=20",
	"FLOAT":    "type=FLOAT",
	"DOUBLE":   "type=DOUBLE",
	"KEY":      "type=BIGINT,size=20,unsigned=1,null=0",
	"TS":       "type=TIMESTAMP,size=6",
	"DATE":     "type=DATE",
	"TEXT":     "type=TEXT",
	"LONGTEXT": "type=LONGTEXT",
	"JSON":     "type=LONGTEXT,format=json",
	"CURRENCY": "type=CHAR,size=5,default=USD,collation=latin1_general_ci",
	"COUNTRY":  "type=CHAR,size=3,default=US,collation=latin1_general_ci",
	"LANGUAGE": "type=CHAR,size=5,default=en-US,collation=latin1_general_ci,validator=language",
	"IP":       "type=VARCHAR,size=39,collation=latin1_general_ci",
	"CIDR":     "type=VARCHAR,size=43,collation=latin1_general_ci",
	"SHA1":     "type=CHAR,size=40,collation=latin1_general_ci",
	"SHA256":   "type=CHAR,size=64,collation=latin1_general_ci",

	// based on types
	"xuid.XUID":       "import=UUID,null=0",
	"*xuid.XUID":      "import=UUID,null=1", // nullable
	"time.Time":       "import=DATETIME,null=0",
	"*time.Time":      "import=DATETIME,null=1",
	"uint64":          "type=BIGINT,size=20,unsigned=1,null=0",
	"int64":           "type=BIGINT,size=21,unsigned=0,null=0",
	"*uint64":         "type=BIGINT,size=20,unsigned=1,null=1",
	"*int64":          "type=BIGINT,size=21,unsigned=0,null=1",
	"float64":         "type=DOUBLE,null=0",
	"*float64":        "type=DOUBLE,null=1",
	"bool":            "type=TINYINT,size=1,null=0",
	"*bool":           "type=TINYINT,size=1,null=1",
	"psql.Set":        "type=SET,null=0",
	"*psql.Set":       "type=SET,null=1",
	"Stamp+time.Time": "import=TS", // for time.Time fields named "Stamp"
}

func DefineMagicType(typ string, definition string) {
	if _, found := magicTypes[typ]; found {
		panic(fmt.Sprintf("multiple definitions of type %s", typ))
	}
	magicTypes[typ] = definition
}

func DefineMagicTypeEngine(e Engine, typ string, definition string) {
	if _, found := magicEngineTypes[e][typ]; found {
		panic(fmt.Sprintf("multiple definitions of type %s", typ))
	}
	magicEngineTypes[e][typ] = definition
}
