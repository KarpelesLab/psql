package psql

type Engine int

const (
	EngineUnknown Engine = iota
	EngineMySQL
	EnginePostgreSQL
	EngineSQLite
)

func (e Engine) String() string {
	switch e {
	case EngineMySQL:
		return "MySQL Engine"
	case EnginePostgreSQL:
		return "PostgreSQL Engine"
	case EngineSQLite:
		return "SQLite Engine"
	default:
		return "Unknown Engine"
	}
}
