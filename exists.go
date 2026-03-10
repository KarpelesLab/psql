package psql

// Exists creates an EXISTS (subquery) condition for use in WHERE clauses:
//
//	psql.B().Select().From("users").Where(
//	    psql.Exists(psql.B().Select(psql.Raw("1")).From("orders").Where(
//	        psql.Equal(psql.F("orders.user_id"), psql.F("users.id")),
//	    )),
//	)
//	// → ... WHERE EXISTS (SELECT 1 FROM "orders" WHERE "orders"."user_id"="users"."id")
func Exists(sub *QueryBuilder) EscapeValueable {
	return &existsExpr{sub: sub}
}

// NotExists creates a NOT EXISTS (subquery) condition for use in WHERE clauses:
//
//	psql.B().Select().From("channels").Where(
//	    psql.NotExists(psql.B().Select(psql.Raw("1")).From("subscriptions").Where(
//	        psql.Equal(psql.F("subscriptions.channel_id"), psql.F("channels.id")),
//	    )),
//	)
func NotExists(sub *QueryBuilder) EscapeValueable {
	return &existsExpr{sub: sub, not: true}
}

type existsExpr struct {
	sub *QueryBuilder
	not bool
}

func (e *existsExpr) EscapeValue() string {
	return e.escapeValueCtx(nil)
}

func (e *existsExpr) escapeValueCtx(ctx *renderContext) string {
	subSQL := escapeCtx(ctx, e.sub) // renders as "(SELECT ...)"
	if e.not {
		return "NOT EXISTS " + subSQL
	}
	return "EXISTS " + subSQL
}
