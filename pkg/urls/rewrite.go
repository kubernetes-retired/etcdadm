package urls

import "strings"

func RewriteScheme(in []string, findPrefix, newPrefix string) []string {
	var out []string
	for _, s := range in {
		if strings.HasPrefix(s, findPrefix) {
			s = newPrefix + strings.TrimPrefix(s, findPrefix)
		}
		out = append(out, s)
	}
	return out
}
