package psql

// DefaultBackend is the global backend used when no backend is attached to the context.
// Set it with [Init], or assign directly.
var DefaultBackend *Backend

// Init creates a new [Backend] using the given DSN and sets it as [DefaultBackend].
// See [New] for supported DSN formats.
func Init(dsn string) error {
	be, err := New(dsn)
	if err != nil {
		return err
	}
	DefaultBackend = be
	return nil
}
