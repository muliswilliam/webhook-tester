// Package view renders the app's HTML pages and the partials streamed to
// them over SSE.
package view

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
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
	"headerEditor": func(id string, headers map[string]any) HeaderEditor {
		return HeaderEditor{ID: id, Headers: headers}
	},
	"forwardURLField": func(id string, value *string) ForwardURLField {
		f := ForwardURLField{ID: id}
		if value != nil {
			f.Value = *value
		}
		return f
	},
	"newWebhookFields":     NewWebhookFields,
	"editWebhookFields":    EditWebhookFields,
	"maxForwardURLLength":  func() int { return models.MaxForwardURLLength },
	"deliveryStatus":       deliveryStatus,
	"deliveryBadge":        NewDeliveryBadge,
	"formatDuration":       formatDuration,
	"fieldValues":          models.FieldValues,
	"maxDeliveryBodyKiB":   func() int { return models.MaxDeliveryResponseBody >> 10 },
	"replayTargetEndpoint": func() models.ReplayTarget { return models.ReplayTargetEndpoint },
	"replayTargetForward":  func() models.ReplayTarget { return models.ReplayTargetForward },
	"replayControl": func(requestID string, forwardURL string, csrfField template.HTML, prominent bool) ReplayControl {
		return ReplayControl{RequestID: requestID, ForwardURL: forwardURL, CSRFField: csrfField, Prominent: prominent}
	},
	// jsString quotes s as a JavaScript string, for a Datastar expression.
	"jsString": func(s string) string {
		b, _ := json.Marshal(s)
		return string(b)
	},
	"forwardToSignal": func() string { return ForwardToSignal },
}

// ForwardToSignal names the page signal holding the forward URL of the
// webhook the page shows, "" if it doesn't forward. The replay controls
// follow it, and the stream updates it when the webhook's settings are
// saved, so controls already on the page offer a replay to the forward URL
// exactly when the webhook forwards: never once it is cleared, and as soon
// as it is set, even on a guest webhook claimed since the page loaded.
const ForwardToSignal = "forwardTo"

// ReplayControl is the data of the "replay-control" template: the replay
// buttons of a captured request.
type ReplayControl struct {
	RequestID string
	// ForwardURL is the webhook's forward URL if it forwards, "" otherwise.
	// The control is rendered for it, then follows the ForwardToSignal.
	ForwardURL string
	CSRFField  template.HTML
	// Prominent makes the main replay the page's primary button.
	Prominent bool
}

// HeaderRow is one row of the response header editor.
type HeaderRow struct {
	Name  string
	Value string
}

// HeaderEditor is the response header editor of a webhook form.
type HeaderEditor struct {
	ID      string         // the editor's element ID
	Headers map[string]any // the current response headers, nil for none
}

// WebhookFields is the data of the "webhook-fields" template: the fields
// of the create and edit webhook forms, prefilled from Webhook.
type WebhookFields struct {
	// IDPrefix starts every element ID, keeping the two forms' apart.
	IDPrefix    string
	Webhook     models.Webhook
	ContentType string // the selected content type
	Payload     string
}

// NewWebhookFields are the create form's fields, prefilled with a new
// webhook's defaults. userID is the signed-in user's ID, 0 for a guest, who
// gets the sign-in prompt instead of the forward URL input.
func NewWebhookFields(userID uint) WebhookFields {
	return WebhookFields{
		IDPrefix:    "create_",
		Webhook:     models.Webhook{ResponseCode: models.DefaultResponseCode, UserID: int(userID)},
		ContentType: models.DefaultContentType,
		Payload:     `{"message":"ok"}`,
	}
}

// EditWebhookFields are the edit form's fields, prefilled from wh.
func EditWebhookFields(wh models.Webhook) WebhookFields {
	f := WebhookFields{IDPrefix: "edit_", Webhook: wh, ContentType: models.DefaultContentType}
	if wh.ContentType != nil && *wh.ContentType != "" {
		f.ContentType = *wh.ContentType
	}
	if wh.Payload != nil {
		f.Payload = *wh.Payload
	}
	return f
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
