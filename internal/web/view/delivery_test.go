package view

import (
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"webhook-tester/internal/models"
)

func ptr[T any](v T) *T { return &v }

// answered is a delivery the target answered with code.
func answered(code int) models.Delivery {
	return models.Delivery{StatusCode: &code, Outcome: models.DeliveryOutcomeForStatus(code)}
}

func TestDeliveryStatus(t *testing.T) {
	for name, tc := range map[string]struct {
		delivery models.Delivery
		want     DeliveryStatus
	}{
		"2xx":     {answered(200), DeliveryStatus{Label: "200", Tone: "success", Title: "200 OK"}},
		"1xx":     {answered(101), DeliveryStatus{Label: "101", Tone: "success", Title: "101 Switching Protocols"}},
		"3xx":     {answered(302), DeliveryStatus{Label: "302", Tone: "neutral", Title: "302 Found"}},
		"4xx":     {answered(401), DeliveryStatus{Label: "401", Tone: "warning", Title: "401 Unauthorized"}},
		"5xx":     {answered(503), DeliveryStatus{Label: "503", Tone: "danger", Title: "503 Service Unavailable"}},
		"unknown": {answered(599), DeliveryStatus{Label: "599", Tone: "danger", Title: "599"}},
		"network error": {
			models.Delivery{Outcome: models.DeliveryOutcomeError, Error: ptr("connection refused")},
			DeliveryStatus{Label: "Error", Tone: "danger", Title: "Not delivered: connection refused"},
		},
		"blocked": {
			models.Delivery{Outcome: models.DeliveryOutcomeBlocked, Error: ptr("destination not allowed: 10.0.0.1 is a private or reserved address")},
			DeliveryStatus{Label: "Blocked", Tone: "danger", Title: "Not delivered: destination not allowed: 10.0.0.1 is a private or reserved address"},
		},
		"dropped": {
			models.Delivery{Outcome: models.DeliveryOutcomeDropped, Error: ptr("forwarding queue full")},
			DeliveryStatus{Label: "Dropped", Tone: "danger", Title: "Not delivered: forwarding queue full"},
		},
		"no outcome": {models.Delivery{}, DeliveryStatus{Label: "Error", Tone: "danger", Title: "Not delivered"}},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, deliveryStatus(tc.delivery))
		})
	}
}

func TestFormatDuration(t *testing.T) {
	assert.Equal(t, "0 ms", formatDuration(0))
	assert.Equal(t, "999 ms", formatDuration(999))
	assert.Equal(t, "1.0 s", formatDuration(1000))
	assert.Equal(t, "10.3 s", formatDuration(10260))
}

func testDeliveries() []models.Delivery {
	started := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)
	return []models.Delivery{
		{
			ID: "del-replay", RequestID: "req-1", Trigger: models.DeliveryTriggerReplay,
			TargetURL: "https://hooks.example.com/stripe", Outcome: models.DeliveryOutcome5xx, StatusCode: ptr(500), DurationMs: 1234,
			ResponseHeaders: datatypes.JSONMap{"X-Handler": "orders", "Set-Cookie": []any{"a=1", "b=2"}},
			ResponseBody:    `{"error":"boom"}`, ResponseBodyTruncated: true,
			StartedAt: started.Add(time.Minute),
		},
		{
			ID: "del-auto", RequestID: "req-1", Trigger: models.DeliveryTriggerAuto,
			TargetURL: "https://hooks.example.com/stripe", Outcome: models.DeliveryOutcomeError, Error: ptr("connection refused"), DurationMs: 3,
			StartedAt: started,
		},
	}
}

func TestRenderPartialMainRequestRowDeliveries(t *testing.T) {
	wr := models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "POST", Deliveries: testDeliveries()}
	html, err := RenderRequestPartial("main-request-row", testRequestRow{Request: wr, ForwardURL: "https://hooks.example.com/stripe"})
	require.NoError(t, err)

	// The badge shows the latest delivery.
	badge := between(t, html, `id="delivery-badge-req-1"`, "</svg>")
	assert.Contains(t, badge, "tone-danger")
	assert.Contains(t, badge, `title="Last delivery: 500 Internal Server Error"`)

	// The list shows every delivery, newest first.
	list := html[strings.Index(html, `id="delivery-list-req-1"`):]
	assert.Less(t, strings.Index(list, `id="delivery-del-replay"`), strings.Index(list, `id="delivery-del-auto"`))
	assert.Contains(t, list, "Replay")
	assert.Contains(t, list, "Auto")
	assert.Contains(t, list, "1.2 s")
	assert.Contains(t, list, "2026-10-01 09:31:00 UTC")
	assert.Contains(t, list, "500 Internal Server Error")
	assert.Contains(t, list, "&#34;error&#34;: &#34;boom&#34;", "formatted JSON response body")
	assert.Contains(t, list, "Truncated to the first 64 KiB")
	assert.Contains(t, list, "X-Handler")
	assert.Regexp(t, `<span class="block">a=1</span>\s*<span class="block">b=2</span>`, list, "a repeated header shows one value per line")
	assert.Contains(t, list, "connection refused")

	// Both replay targets are offered.
	assert.Contains(t, html, `name="target" value="endpoint"`)
	assert.Contains(t, html, `name="target" value="forward"`)
	forward := between(t, html, `value="forward"`, "</button>")
	assert.Contains(t, forward, `class="btn-secondary"`, "a row's replays are secondary")
	assert.Contains(t, forward, `title="Send to https://hooks.example.com/stripe"`)
	assert.Contains(t, forward, "Replay to forward URL")
	assert.Contains(t, html, `data-show="!!$forwardTo"`, "the forward replay follows the forward URL")
}

// A webhook that doesn't forward, guest or not, renders the forward replay
// hidden, so the forwardTo signal can show it once a forward URL is set.
func TestRenderPartialMainRequestRowWithoutForwarding(t *testing.T) {
	wr := models.WebhookRequest{ID: "req-1", WebhookID: "wh-1", Method: "POST"}
	html, err := RenderRequestPartial("main-request-row", testRequestRow{Request: wr})
	require.NoError(t, err)

	// Empty targets for live deliveries, but no badge or delivery.
	assert.Contains(t, html, `id="delivery-badge-req-1"`)
	assert.Contains(t, html, `id="delivery-list-req-1"`)
	assert.NotContains(t, html, "delivery-badge tone-")
	assert.NotContains(t, html, "delivery-item")

	endpoint := between(t, html, `value="endpoint"`, "</button>")
	assert.Contains(t, endpoint, "Replay request")
	assert.Contains(t, endpoint, `data-text="$forwardTo ? 'Replay to endpoint' : 'Replay request'"`)
	forward := between(t, html, `action="/requests/req-1/replay"
    style="display: none"`, "</button>")
	assert.Contains(t, forward, `value="forward"`)
	assert.Contains(t, forward, `data-show="!!$forwardTo"`)
}

func TestRenderPartialDeliveryBadgeAndItem(t *testing.T) {
	d := testDeliveries()[1]
	badge, err := RenderRequestPartial("delivery-badge", DeliveryBadge{RequestID: "req-1", Delivery: &d})
	require.NoError(t, err)
	assert.Contains(t, badge, `id="delivery-badge-req-1"`)
	assert.Contains(t, badge, "Error")
	assert.Contains(t, badge, "Last delivery: Not delivered: connection refused")

	item, err := RenderRequestPartial("delivery-item", d)
	require.NoError(t, err)
	assert.Contains(t, item, `id="delivery-del-auto"`)
	assert.Contains(t, item, "connection refused")
	assert.NotContains(t, item, "<details", "a delivery without an answer has nothing to expand")
}

func renderRequestPage(t *testing.T, webhook models.Webhook, req models.WebhookRequest) string {
	t.Helper()
	return renderRequestPageWithSidebar(t, []models.Webhook{webhook}, webhook, req)
}

// renderRequestPageWithSidebar is renderRequestPage with the given webhooks
// in the sidebar.
func renderRequestPageWithSidebar(t *testing.T, sidebar []models.Webhook, webhook models.Webhook, req models.WebhookRequest) string {
	t.Helper()
	data := struct {
		ID        string
		Year      int
		User      models.User
		Webhooks  []models.Webhook
		Webhook   *models.Webhook
		Request   *models.WebhookRequest
		CanManage bool
		CSRFField template.HTML
	}{
		ID:        req.ID,
		Webhooks:  sidebar,
		Webhook:   &webhook,
		Request:   &req,
		CSRFField: template.HTML(`<input type="hidden">`),
	}
	w := httptest.NewRecorder()
	RenderHTML(w, httptest.NewRequest("GET", "/", nil), "request", data)
	require.Equal(t, 200, w.Code)
	return w.Body.String()
}

func TestRenderHTMLRequestDeliveries(t *testing.T) {
	webhook := newTestWebhook()
	webhook.UserID = 7
	webhook.ForwardURL = ptr("https://hooks.example.com/stripe")
	req := webhook.Requests[0]

	t.Run("forwarding webhook", func(t *testing.T) {
		req.Deliveries = testDeliveries()
		body := renderRequestPage(t, webhook, req)
		assert.Contains(t, body, "Deliveries")
		assert.Contains(t, body, `id="delivery-del-replay"`)
		assert.Contains(t, body, `name="target" value="forward"`)
		forward := between(t, body, `value="forward"`, "</button>")
		assert.Contains(t, forward, `class="btn-primary"`, "the page's main action")
		assert.Contains(t, forward, `title="Send to https://hooks.example.com/stripe"`)
		assert.Contains(t, body, `data-signals:forward-to="&#34;https://hooks.example.com/stripe&#34;"`)
	})

	t.Run("forwarding webhook, request not forwarded yet", func(t *testing.T) {
		req.Deliveries = nil
		body := renderRequestPage(t, webhook, req)
		assert.Contains(t, body, `id="delivery-list-req-1"`)
		assert.Contains(t, body, "Not forwarded yet")
	})

	t.Run("webhook that doesn't forward", func(t *testing.T) {
		guest := newTestWebhook()
		guest.ForwardURL = ptr("https://hooks.example.com/stripe")
		req.Deliveries = nil
		body := renderRequestPage(t, guest, req)
		assert.NotContains(t, body, "Deliveries")
		assert.Contains(t, body, `data-signals:forward-to="&#34;&#34;"`)
		assert.Regexp(t, `style="display: none"\s+data-show="!!\$forwardTo"\s*>\s*<input type="hidden">\s*<input type="hidden" name="target" value="forward"`,
			body, "the forward replay is hidden until a forward URL is set")
		endpoint := between(t, body, `value="endpoint"`, "</button>")
		assert.Contains(t, endpoint, `class="btn-primary"`)
		assert.Contains(t, endpoint, "Replay request")
	})
}

// The request page's stream only updates that request's deliveries.
func TestRenderHTMLRequestStreamScope(t *testing.T) {
	webhook := newTestWebhook()
	body := renderRequestPage(t, webhook, webhook.Requests[0])

	literal := jsStringLiteralAfter(t, body, "@get('", 0)
	assert.Equal(t, "/webhook-stream/"+webhook.ID+"?since="+models.LatestCursor(webhook.Requests).String()+"&request=req-1", literal)
	assert.Equal(t, 1, strings.Count(body, "/webhook-stream/"+webhook.ID+"?"))
}

// The request page of a webhook missing from the sidebar opens the
// webhook's stream on its own; see TestRenderHTMLHomeUnlistedWebhookStream.
func TestRenderHTMLRequestUnlistedWebhookStream(t *testing.T) {
	webhook := newTestWebhook()
	body := renderRequestPageWithSidebar(t, []models.Webhook{{ID: "wh-own"}}, webhook, webhook.Requests[0])

	literal := jsStringLiteralAfter(t, body, "@get('/webhook-stream/"+webhook.ID, 0)
	assert.Equal(t, "?since="+models.LatestCursor(webhook.Requests).String()+"&request=req-1&unlisted", literal)
	assert.Equal(t, 1, strings.Count(body, "/webhook-stream/"+webhook.ID+"?"))
}

// between returns the part of s from the first from up to the next to.
func between(t *testing.T, s, from, to string) string {
	t.Helper()
	start := strings.Index(s, from)
	require.NotEqual(t, -1, start, "%q not found in:\n%s", from, s)
	end := strings.Index(s[start:], to)
	require.NotEqual(t, -1, end, "%q not found after %q in:\n%s", to, from, s)
	return s[start : start+end]
}
