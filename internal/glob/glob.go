package glob

// Match reports whether text matches pattern using SQLite GLOB syntax:
// "*" matches any sequence, "?" matches one character, and "[...]" matches
// one character from a set, where a leading "^" negates the set.
func Match(pattern, text string) bool {
	p := []rune(pattern)
	t := []rune(text)

	pi, ti := 0, 0
	starP, starT := -1, 0
	for ti < len(t) {
		if pi < len(p) {
			switch p[pi] {
			case '*':
				starP, starT = pi, ti
				pi++
				continue
			case '?':
				pi++
				ti++
				continue
			case '[':
				if ok, next := matchSet(p, pi, t[ti]); ok {
					pi = next
					ti++
					continue
				}
			default:
				if p[pi] == t[ti] {
					pi++
					ti++
					continue
				}
			}
		}

		if starP < 0 {
			return false
		}
		starT++
		pi, ti = starP+1, starT
	}

	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

func matchSet(p []rune, start int, c rune) (bool, int) {
	i := start + 1
	negate := false
	if i < len(p) && p[i] == '^' {
		negate = true
		i++
	}

	matched := false
	if i < len(p) && p[i] == ']' {
		matched = c == ']'
		i++
	}

	var prior rune
	hasPrior := false
	for {
		if i >= len(p) {
			return false, 0
		}
		if p[i] == ']' {
			break
		}
		if p[i] == '-' && hasPrior && i+1 < len(p) && p[i+1] != ']' {
			if prior <= c && c <= p[i+1] {
				matched = true
			}
			hasPrior = false
			i += 2
		} else {
			if p[i] == c {
				matched = true
			}
			prior, hasPrior = p[i], true
			i++
		}
	}

	return matched != negate, i + 1
}
