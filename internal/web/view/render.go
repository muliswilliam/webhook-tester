// Package view renders the app's HTML pages and the partials streamed to
// them over SSE.
package view

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"webhook-tester/internal/models"
	"webhook-tester/internal/utils"
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
	// flash is bound per render to the request's queued notice; see withFlash.
	"flash":            func() *utils.Flash { return nil },
	"minResponseCode":  func() int { return models.MinResponseCode },
	"maxResponseCode":  func() int { return models.MaxResponseCode },
	"maxResponseDelay": func() int { return models.MaxResponseDelay },
	"headerRow": func(name string, value any) HeaderRow {
		return HeaderRow{Name: name, Value: fmt.Sprint(value)}
	},
	"forwardURLField": func(id string, value *string) ForwardURLField {
		f := ForwardURLField{ID: id}
		if value != nil {
			f.Value = *value
		}
		return f
	},
	"maxForwardURLLength": func() int { return models.MaxForwardURLLength },
	"deliveryStatus":      deliveryStatus,
	"deliveryBadge":       deliveryBadge,
	"formatDuration":      formatDuration,
	"fieldValue":          models.FieldValue,
	"maxDeliveryBodyKiB":  func() int { return models.MaxDeliveryResponseBody >> 10 },
}

// HeaderRow is one row of the response header editor.
type HeaderRow struct {
	Name  string
	Value string
}

// ForwardURLField is the forward URL input of a webhook form.
type ForwardURLField struct {
	ID    string // the input's element ID
	Value string // the current forward URL, "" for none
}

// withFlash pops the request's queued notice and binds it to the "flash"
// template function, so every page can show it without threading it through
// each handler's data.
func withFlash(tmpl *template.Template, w http.ResponseWriter, r *http.Request) *template.Template {
	f := utils.PopFlash(w, r)
	return tmpl.Funcs(template.FuncMap{"flash": func() *utils.Flash { return f }})
}

func RenderHTML(w http.ResponseWriter, r *http.Request, tmplName string, data interface{}) {
	renderPage(w, r, http.StatusOK, data, "base.html", "layout.html", "request_row.html", tmplName+".html")
}

func RenderHTMLWithoutLayout(w http.ResponseWriter, r *http.Request, tmplName string, data interface{}) {
	renderPage(w, r, http.StatusOK, data, "base.html", tmplName+".html")
}

// RenderNotFound renders the 404 page.
func RenderNotFound(w http.ResponseWriter, r *http.Request) {
	renderPage(w, r, http.StatusNotFound, nil, "base.html", "not-found.html")
}

func renderPage(w http.ResponseWriter, r *http.Request, status int, data interface{}, files ...string) {
	tmpl := template.Must(template.New("base.html").Funcs(funcMap).ParseFS(templates.Templates, files...))
	tmpl = withFlash(tmpl, w, r)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		logger.Printf("error rendering %v: %v", files, err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		logger.Printf("error writing %v: %v", files, err)
	}
}

// requestRowTmpl is request_row.html parsed once and reused by every
// RenderRequestPartial call. This function is a hot path - it's invoked
// 2-3 times per broadcast event, once per SSE subscriber to a webhook - so
// re-parsing the template from the embedded FS on every call would be far
// more wasteful here than it is for the once-per-page-load renders in
// RenderHTML/RenderHTMLWithoutLayout. ExecuteTemplate is safe for concurrent
// use once parsing is done, so the cached template can be shared across
// goroutines without locking.
var (
	requestRowTmpl     *template.Template
	requestRowTmplOnce sync.Once
	requestRowTmplErr  error
)

// RenderRequestPartial renders a single named template (defined in request_row.html)
// to a string, for use outside a full-page response - e.g. an SSE-pushed DOM patch.
func RenderRequestPartial(tmplName string, data interface{}) (string, error) {
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
