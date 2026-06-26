// shlex.go: a minimal POSIX-ish command splitter for the `cmd` string in a
// project's .qwok.toml.
//
// Why this exists: portless injects framework-specific --port/--host flags by
// inspecting the real argv of the command it wraps. So we must pass the dev
// command as separate tokens (`portless myapp npm run dev`), NOT wrapped in
// `sh -c "..."` — wrapping would hide the real command from portless and break
// its port injection. This splitter turns "npm run dev" into ["npm","run","dev"]
// while honoring simple single/double quotes for args that contain spaces.
//
// It is intentionally small: dev commands are simple. It does not implement
// shell operators (&&, |, $VAR, globbing); a command needing those should be
// put behind an npm/make script and referenced by name.
package app

import (
	"fmt"
	"strings"
)

// splitCommand tokenizes s with whitespace as the separator, honoring '...' and
// "..." quoting. An unterminated quote is an error.
func splitCommand(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inToken := false
	var quote rune // 0, '\'' or '"'

	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
			inToken = true
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case r == ' ' || r == '\t' || r == '\n':
			if inToken {
				tokens = append(tokens, cur.String())
				cur.Reset()
				inToken = false
			}
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	if inToken {
		tokens = append(tokens, cur.String())
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return tokens, nil
}
