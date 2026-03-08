package psql

// Scope is a reusable query modifier function. Scopes can add WHERE conditions,
// ORDER BY, LIMIT, JOINs, or any other SQL clause to a [QueryBuilder].
//
// Define scopes as package-level variables or functions:
//
//	var Active Scope = func(q *QueryBuilder) *QueryBuilder {
//	    return q.Where(map[string]any{"Status": "active"})
//	}
//
//	func RecentN(n int) Scope {
//	    return func(q *QueryBuilder) *QueryBuilder {
//	        return q.OrderBy(S("CreatedAt", "DESC")).Limit(n)
//	    }
//	}
//
// Use with [Fetch], [Get], [Count], or the query builder:
//
//	users, err := psql.Fetch[User](ctx, nil, psql.WithScope(Active, RecentN(10)))
//	rows, err := psql.B().Select().From("users").Apply(Active).RunQuery(ctx)
type Scope func(q *QueryBuilder) *QueryBuilder

// WithScope returns a [FetchOptions] that applies the given scopes to the query.
func WithScope(scopes ...Scope) *FetchOptions {
	return &FetchOptions{Scopes: scopes}
}
