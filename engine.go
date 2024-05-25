package psql

type Engine int

const (
	EngineUnknown Engine = iota
	EngineMySQL
	EnginePostgreSQL
)

func (e Engine) String() string {
	switch e {
	case EngineMySQL:
		return "MySQL Engine"
	case EnginePostgreSQL:
		return "PostreSQL Engine"
	default:
		return "Unknown Engine"
	}
}
