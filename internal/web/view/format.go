package view

import (
	"bytes"
	"encoding/json"
)

// prettyJSON returns body indented for display, or "" when body isn't JSON or
// indenting wouldn't change it.
func prettyJSON(body string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(body), "", "  "); err != nil || buf.String() == body {
		return ""
	}
	return buf.String()
}
