package psql

import "strings"

func parseTagData(tag string) (string, map[string]string) {
	data := strings.Split(tag, ",")
	col := data[0]
	attrs := make(map[string]string)

	for _, v := range data[1:] {
		p := strings.IndexByte(v, '=')
		if p == -1 {
			attrs[v] = ""
			continue
		}
		attrs[v[:p]] = v[p+1:]
	}

	return col, attrs
}
