package psql

import (
	"strings"
	"unicode"
)

var FormatTableName = formatCamelSnakeCase

// format to Camel_Snake_Case
func formatCamelSnakeCase(name string) string {
	b := &strings.Builder{}

	for n, c := range name {
		if n == 0 {
			b.WriteRune(unicode.ToUpper(c))
			continue
		}
		if !unicode.IsLetter(c) {
			if unicode.IsNumber(c) {
				b.WriteRune(c)
			}
			continue
		}
		if unicode.IsUpper(c) {
			b.WriteByte('_')
		}
		b.WriteRune(c)
	}

	return b.String()
}
