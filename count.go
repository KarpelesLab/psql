package psql

import (
	"context"
	"log"
	"os"
	"strings"
)

func Count[T any](ctx context.Context, where map[string]any) (int, error) {
	return Table[T]().Count(ctx, where)
}

func (t *TableMeta[T]) Count(ctx context.Context, where map[string]any) (int, error) {
	// simplified get
	req := "SELECT COUNT(1) FROM " + QuoteName(t.table)
	var params []any
	if where != nil {
		var whQ []string
		for k, v := range where {
			switch rv := v.(type) {
			case []string:
				// IN (...)
				repQ := make([]string, len(rv))
				for n := range repQ {
					repQ[n] = "?"
				}
				whQ = append(whQ, QuoteName(k)+" IN ("+strings.Join(repQ, ",")+")")
				for _, sv := range rv {
					params = append(params, sv)
				}
			case []any:
				// IN (...)
				repQ := make([]string, len(rv))
				for n := range repQ {
					repQ[n] = "?"
				}
				whQ = append(whQ, QuoteName(k)+" IN ("+strings.Join(repQ, ",")+")")
				for _, sv := range rv {
					params = append(params, sv)
				}
			default:
				whQ = append(whQ, QuoteName(k)+"=?")
				params = append(params, v)
			}
		}
		if len(whQ) > 0 {
			req += " WHERE " + strings.Join(whQ, " AND ")
		}
	}

	// run query
	rows, err := doQueryContext(ctx, req, params...)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return 0, &Error{Query: req, Err: err}
	}
	defer rows.Close()

	if !rows.Next() {
		// should not happen with COUNT(1)
		return 0, os.ErrNotExist
	}

	var res int
	err = rows.Scan(&res)

	return res, err
}
