package psql

// parseTagData converts a tag into a column name (or empty if none) and parameters
//
// It can parse various syntaxes:
//
// Col string `sql:",size=foo,baz=bar"`
// Col string `sql:",type=enum,values='a,b,c'"`
func parseTagData(tag string) (string, map[string]string) {
	return internalParse(tag, true)
}

func parseAttrs(tag string) map[string]string {
	// parse with first=false so we don't take first argument as column name
	_, res := internalParse(tag, false)
	return res
}

func internalParse(tag string, first bool) (string, map[string]string) {
	state := 0
	col := ""
	attrs := make(map[string]string)
	var a, b []rune
	var q rune

	// read rune by rune
	for _, c := range tag {
		switch state {
		case 0: // reading key
			switch c {
			case '=':
				state = 1
			case ',':
				if first {
					col = string(a)
					first = false
				} else {
					attrs[string(a)] = ""
				}
				a = nil
			default:
				a = append(a, c)
			}
		case 1: // reading value
			switch c {
			case '\'', '"':
				q = c
				state = 2
			case ',':
				attrs[string(a)] = string(b)
				a, b = nil, nil
				state = 0
			default:
				b = append(b, c)
			}
		case 2: // in quotes
			if c == q {
				// out of quote
				state = 1
			} else {
				b = append(b, c)
			}
		}
	}

	if len(a) > 0 {
		if first {
			col = string(a)
		} else {
			// add final value
			attrs[string(a)] = string(b)
		}
	}

	//log.Printf("parsed %s into %s => %v", tag, col, attrs)

	return col, attrs
}
