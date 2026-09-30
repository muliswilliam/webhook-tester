package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
	"webhook-tester/internal/models"
	"webhook-tester/internal/web/templates"
)

// funcMap is shared by every template so partials rendered on their own (e.g. for SSE
// patches) resolve the same helpers as full-page renders.
//
// signalKey/signalKeyJS derive a JS-identifier-safe suffix from an opaque ID (which may
// contain "-" from nanoid's alphabet) for use in per-row Datastar signal names like
// `open_{{ signalKey .ID }}`. Go's html/template treats any "data-on:*" attribute as a
// JavaScript context (it strips the "data-" prefix and matches the remaining "on" prefix),
// so values interpolated there get JSON-string-quoted unless marked as trusted JS:
// signalKeyJS must be used inside data-on:* attributes, signalKey everywhere else.
var funcMap = template.FuncMap{
	"signalKey": func(id string) string {
		return hex.EncodeToString([]byte(id))
	},
	"signalKeyJS": func(id string) template.JS {
		return template.JS(hex.EncodeToString([]byte(id)))
	},
	"asset":        assetURL,
	"prettyJSON":   prettyJSON,
	"streamCursor": func(requests []models.WebhookRequest) string { return models.LatestCursor(requests).String() },
}

// prettyJSON returns body indented for display, or "" when body isn't JSON or
// indenting wouldn't change it.
func prettyJSON(body string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(body), "", "  "); err != nil || buf.String() == body {
		return ""
	}
	return buf.String()
}

// staticDir is where the server's /static/ route serves files from.
const staticDir = "static"

type assetVersion struct {
	modTime time.Time
	size    int64
	hash    string
}

var assetVersions sync.Map // static path -> assetVersion

// assetURL returns the /static/ URL for path with a content-hash query
// string, so browsers refetch an asset exactly when its content changes.
// Hashes are cached until the file's size or mtime changes.
func assetURL(path string) string {
	url := "/static/" + path
	file := filepath.Join(staticDir, path)
	info, err := os.Stat(file)
	if err != nil {
		log.Printf("asset %s: %v", path, err)
		return url
	}
	if v, ok := assetVersions.Load(path); ok {
		if v := v.(assetVersion); v.modTime.Equal(info.ModTime()) && v.size == info.Size() {
			return url + "?v=" + v.hash
		}
	}
	content, err := os.ReadFile(file)
	if err != nil {
		log.Printf("asset %s: %v", path, err)
		return url
	}
	sum := sha256.Sum256(content)
	v := assetVersion{modTime: info.ModTime(), size: info.Size(), hash: hex.EncodeToString(sum[:])[:12]}
	assetVersions.Store(path, v)
	return url + "?v=" + v.hash
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
