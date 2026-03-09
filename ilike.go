package psql

// CILike creates a case-insensitive [Like] condition:
//
//	psql.CILike(psql.F("name"), "john%")
func CILike(field any, pattern string) *Like {
	return &Like{Field: field, Like: pattern, CaseInsensitive: true}
}
