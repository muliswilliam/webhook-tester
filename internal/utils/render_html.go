package utils

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"webhook-tester/internal/web/templates"
)

// funcMap is shared by every template so partials rendered on their own (e.g. for SSE
// patches) resolve the same helpers as full-page renders.
//
// sig/sigJS derive a JS-identifier-safe suffix from an opaque ID (which may contain "-"
// from nanoid's alphabet) for use in per-row Datastar signal names like
// `open_{{ sig .ID }}`. Go's html/template treats any "data-on:*"/"data-on-*" attribute
// as a JavaScript context (it strips the "data-" prefix and matches the remaining "on"
// prefix), so values interpolated there get JSON-string-quoted unless explicitly marked
// as trusted JS via template.JS. sigJS must be used inside data-on:* attributes; sig is
// for every other (plain HTML) context, e.g. data-signals/data-show/data-text/ids.
var funcMap = template.FuncMap{
	"sig": func(id string) string {
		return hex.EncodeToString([]byte(id))
	},
	"sigJS": func(id string) template.JS {
		return template.JS(hex.EncodeToString([]byte(id)))
	},
	// dict builds a map from alternating string-key/value pairs, letting a
	// {{ template }} call pass more than one value as its dot.
	"dict": func(pairs ...interface{}) (map[string]interface{}, error) {
		if len(pairs)%2 != 0 {
			return nil, fmt.Errorf("dict: expected an even number of arguments, got %d", len(pairs))
		}
		m := make(map[string]interface{}, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			key, ok := pairs[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict: key %v is not a string", pairs[i])
			}
			m[key] = pairs[i+1]
		}
		return m, nil
	},
}

func RenderHtml(w http.ResponseWriter, r *http.Request, tmplName string, data interface{}) {
	files := []string{
		"base.html",
		"layout.html",
		"request_row.html",
		tmplName + ".html",
	}
	tmpl := template.Must(template.New("base.html").Funcs(funcMap).ParseFS(templates.Templates, files...))

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func RenderHtmlWithoutLayout(w http.ResponseWriter, r *http.Request, tmplName string, data interface{}) {
	files := []string{
		"base.html",
		tmplName + ".html",
	}
	tmpl := template.Must(template.New("base.html").Funcs(funcMap).ParseFS(templates.Templates, files...))

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// requestRowTmpl is request_row.html parsed once and reused by every
// RenderPartialToString call. This function is a hot path - it's invoked
// 2-3 times per broadcast event, once per SSE subscriber to a webhook - so
// re-parsing the template from the embedded FS on every call would be far
// more wasteful here than it is for the once-per-page-load renders in
// RenderHtml/RenderHtmlWithoutLayout. ExecuteTemplate is safe for concurrent
// use once parsing is done, so the cached template can be shared across
// goroutines without locking.
var (
	requestRowTmpl     *template.Template
	requestRowTmplOnce sync.Once
	requestRowTmplErr  error
)

// RenderPartialToString renders a single named template (defined in request_row.html)
// to a string, for use outside a full-page response - e.g. an SSE-pushed DOM patch.
func RenderPartialToString(tmplName string, data interface{}) (string, error) {
	requestRowTmplOnce.Do(func() {
		requestRowTmpl, requestRowTmplErr = template.New("request_row.html").Funcs(funcMap).ParseFS(templates.Templates, "request_row.html")
	})
	if requestRowTmplErr != nil {
		return "", requestRowTmplErr
	}

	var buf bytes.Buffer
	if err := requestRowTmpl.ExecuteTemplate(&buf, tmplName, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
