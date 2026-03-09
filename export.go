package psql

// export transforms various known types to types easier to handle for the SQL server.
// It delegates to the engine's [Dialect.ExportArg] for engine-specific formatting.
func (e Engine) export(in any, f *StructField) any {
	return e.dialect().ExportArg(in)
}
