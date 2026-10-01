package routers_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"webhook-tester/config"
	"webhook-tester/internal/mailer"
	appMetrics "webhook-tester/internal/metrics"
	"webhook-tester/internal/models"
	"webhook-tester/internal/routers"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"
	"webhook-tester/internal/utils"
)

// allowLoopback is the forwarding policy the tests use to reach their
// loopback test servers.
var allowLoopback = config.Forwarding{AllowPrivateNetworks: true, Timeout: 5 * time.Second, MaxConcurrent: 8}

// forwardingEnv is the webhook and web routers over shared real services
// and an in-memory DB, as the server mounts them.
type forwardingEnv struct {
	db         *gorm.DB
	webhooks   http.Handler // mounted at /webhooks
	web        http.Handler // mounted at /
	webhookSvc *service.WebhookService
	forwarder  *service.Forwarder
	owner      *models.User
	session    *http.Cookie
}

func newForwardingEnv(t *testing.T, cfg config.Forwarding) *forwardingEnv {
	t.Helper()
	return newForwardingEnvAt(t, cfg, testDomain)
}

// newForwardingEnvAt is newForwardingEnv for an instance served at domain,
// where replays to the endpoint are sent.
func newForwardingEnvAt(t *testing.T, cfg config.Forwarding, domain string) *forwardingEnv {
	t.Helper()
	t.Setenv("AUTH_SECRET", "some-32-plus-byte-secret-value!!")
	t.Setenv("DOMAIN", "http://example.com") // the web router's CSRF origin

	db := newAPITestDB(t)
	logger := testLogger()
	authSvc := service.NewAuthService(store.NewGormUserRepo(db, logger), db, "some-32-plus-byte-secret-value!!")
	webhookSvc := service.NewWebhookService(store.NewGormWebookRepo(db, logger), store.NewGormDeliveryRepo(db, logger), domain,
		service.ForwardPolicy{AllowPrivateNetworks: cfg.AllowPrivateNetworks, Resolver: testResolver{}})
	webhookReqSvc := service.NewWebhookRequestService(store.NewGormWebhookRequestRepo(db, logger))
	forwarder := newTestForwarder(t, webhookSvc, cfg)
	metricsRec := &appMetrics.PrometheusRecorder{}

	owner, err := authSvc.Register("owner@example.com", "Passw0rd!", "Owner")
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	require.NoError(t, authSvc.CreateSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), owner))
	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Value != "" {
			session = c
		}
	}
	require.NotNil(t, session, "session cookie")

	return &forwardingEnv{
		db:         db,
		webhooks:   routers.NewWebhookRouter(webhookSvc, webhookReqSvc, authSvc, forwarder, logger, metricsRec),
		web:        routers.NewWebRouter(webhookReqSvc, webhookSvc, authSvc, forwarder, &mailer.LogMailer{Logger: logger}, metricsRec, logger),
		webhookSvc: webhookSvc,
		forwarder:  forwarder,
		owner:      owner,
		session:    session,
	}
}

// createWebhook stores a webhook forwarding to forwardURL, owned by the
// env's user unless guest is set.
func (e *forwardingEnv) createWebhook(t *testing.T, id, forwardURL string, guest bool) {
	t.Helper()
	wh := models.Webhook{ID: id, Title: id, ResponseCode: http.StatusAccepted, ForwardURL: &forwardURL}
	if !guest {
		wh.UserID = int(e.owner.ID)
	}
	require.NoError(t, e.db.Create(&wh).Error)
}

// capture sends a provider request to the webhook endpoint.
func (e *forwardingEnv) capture(t *testing.T, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.webhooks.ServeHTTP(rec, req)
	return rec
}

func (e *forwardingEnv) requests(t *testing.T, webhookID string) []models.WebhookRequest {
	t.Helper()
	var list []models.WebhookRequest
	require.NoError(t, e.db.Where("webhook_id = ?", webhookID).Order("received_at").Find(&list).Error)
	return list
}

// onlyRequest returns the webhook's single captured request.
func (e *forwardingEnv) onlyRequest(t *testing.T, webhookID string) models.WebhookRequest {
	t.Helper()
	list := e.requests(t, webhookID)
	require.Len(t, list, 1)
	return list[0]
}

func (e *forwardingEnv) deliveries(t *testing.T, requestID string) []models.Delivery {
	t.Helper()
	list, err := store.NewGormDeliveryRepo(e.db, testLogger()).ListByRequest(requestID)
	require.NoError(t, err)
	return list
}

// awaitDeliveries waits until the request has n deliveries recorded and
// returns them, newest first.
func (e *forwardingEnv) awaitDeliveries(t *testing.T, requestID string, n int) []models.Delivery {
	t.Helper()
	var list []models.Delivery
	require.Eventually(t, func() bool {
		list = nil
		err := e.db.Where("request_id = ?", requestID).Order("started_at DESC, id DESC").Find(&list).Error
		return err == nil && len(list) >= n
	}, 5*time.Second, 5*time.Millisecond, "waiting for %d deliveries of request %s", n, requestID)
	require.Len(t, list, n)
	return list
}

// awaitDelivery waits for the request's single delivery and returns it.
func (e *forwardingEnv) awaitDelivery(t *testing.T, requestID string) models.Delivery {
	t.Helper()
	return e.awaitDeliveries(t, requestID, 1)[0]
}

// neverDelivered asserts that the request gets no delivery for a while.
func (e *forwardingEnv) neverDelivered(t *testing.T, requestID string) {
	t.Helper()
	assert.Never(t, func() bool {
		var n int64
		return e.db.Model(&models.Delivery{}).Where("request_id = ?", requestID).Count(&n).Error == nil && n > 0
	}, 300*time.Millisecond, 10*time.Millisecond, "request %s got a delivery", requestID)
}

var csrfFieldPattern = regexp.MustCompile(`name="gorilla.csrf.Token" value="([^"]+)"`)

// postForm submits a form to the web router as the signed-in owner, with a
// valid CSRF token, from the referring page.
func (e *forwardingEnv) postForm(t *testing.T, path string, form url.Values, referer string) *httptest.ResponseRecorder {
	t.Helper()
	page := httptest.NewRecorder()
	e.web.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/login", nil))
	m := csrfFieldPattern.FindStringSubmatch(page.Body.String())
	require.NotNil(t, m, "CSRF field on the login page")
	if form == nil {
		form = url.Values{}
	}
	form.Set("gorilla.csrf.Token", m[1])

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", referer)
	for _, c := range page.Result().Cookies() {
		req.AddCookie(c)
	}
	req.AddCookie(e.session)
	rec := httptest.NewRecorder()
	e.web.ServeHTTP(rec, req)
	return rec
}

func flashOf(t *testing.T, rec *httptest.ResponseRecorder) *utils.Flash {
	t.Helper()
	next := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		next.AddCookie(c)
	}
	return utils.PopFlash(httptest.NewRecorder(), next)
}

// receivedRequest is what a forward target saw.
type receivedRequest struct {
	Method      string
	Path        string
	EscapedPath string
	RawQuery    string
	Header      http.Header
	Body        []byte
}

// target is a forward target server recording what it receives.
type target struct {
	*httptest.Server
	mu       sync.Mutex
	received []receivedRequest
}

func newTarget(t *testing.T, respond http.HandlerFunc) *target {
	t.Helper()
	tg := &target{}
	tg.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		tg.mu.Lock()
		tg.received = append(tg.received, receivedRequest{
			Method: r.Method, Path: r.URL.Path, EscapedPath: r.URL.EscapedPath(), RawQuery: r.URL.RawQuery,
			Header: r.Header.Clone(), Body: body,
		})
		tg.mu.Unlock()
		if respond != nil {
			respond(w, r)
		}
	}))
	t.Cleanup(tg.Close)
	return tg
}

func (tg *target) requests() []receivedRequest {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	return append([]receivedRequest(nil), tg.received...)
}

func TestForwarding_RelaysCapturedRequestFaithfully(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handler", "mine")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	env.createWebhook(t, "wh1", tg.URL+"/hooks?token=abc", false)
	sub := env.webhookSvc.Subscribe("wh1")
	defer sub.Close()

	// Non-canonical JSON: re-encoding it would change the bytes, and so
	// break the handler's signature check.
	body := "{\"amount\":  100,\n  \"currency\":\"usd\" }\n"
	rec := env.capture(t, http.MethodPut, "/wh1/orders/42?x=1&y=two", body, map[string]string{
		"Content-Type":     "application/json",
		"Stripe-Signature": "t=1,v1=abc",
		"Connection":       "keep-alive, X-Hop",
		"X-Hop":            "dropped because Connection names it",
		"Keep-Alive":       "timeout=5",
		"Upgrade":          "websocket",
	})
	require.Equal(t, http.StatusAccepted, rec.Code, "the provider gets the configured response")

	captured := env.onlyRequest(t, "wh1")
	d := env.awaitDelivery(t, captured.ID)
	got := tg.requests()
	require.Len(t, got, 1)
	fwd := got[0]
	assert.Equal(t, http.MethodPut, fwd.Method)
	assert.Equal(t, "/hooks/orders/42", fwd.Path)
	query, err := url.ParseQuery(fwd.RawQuery)
	require.NoError(t, err)
	assert.Equal(t, url.Values{"token": {"abc"}, "x": {"1"}, "y": {"two"}}, query)
	assert.Equal(t, body, string(fwd.Body), "the body is relayed byte for byte")
	assert.Equal(t, "application/json", fwd.Header.Get("Content-Type"))
	assert.Equal(t, "t=1,v1=abc", fwd.Header.Get("Stripe-Signature"))
	assert.Equal(t, captured.ID, fwd.Header.Get(service.RequestIDHeader))
	for _, h := range []string{"X-Hop", "Keep-Alive", "Upgrade", "Accept-Encoding", "User-Agent"} {
		assert.NotContains(t, fwd.Header, h)
	}

	assert.Equal(t, models.DeliveryTriggerAuto, d.Trigger)
	assert.Equal(t, "wh1", d.WebhookID)
	assert.Equal(t, tg.URL+"/hooks/orders/42?token=abc&x=1&y=two", d.TargetURL)
	require.NotNil(t, d.StatusCode)
	assert.Equal(t, http.StatusOK, *d.StatusCode)
	assert.Equal(t, models.DeliveryOutcome2xx, d.Outcome)
	assert.Nil(t, d.Error)
	assert.Equal(t, `{"ok":true}`, d.ResponseBody)
	assert.False(t, d.ResponseBodyTruncated)
	assert.Equal(t, "mine", d.ResponseHeaders["X-Handler"])
	assert.False(t, d.StartedAt.IsZero())

	// Subscribers hear about the capture, then the delivery.
	evt := <-sub.Events
	assert.Equal(t, service.EventRequestCaptured, evt.Kind)
	select {
	case evt = <-sub.Events:
		assert.Equal(t, service.EventDeliveryRecorded, evt.Kind)
		assert.Equal(t, d.ID, evt.Delivery.ID)
		assert.Equal(t, captured.ID, evt.Delivery.RequestID)
	case <-time.After(5 * time.Second):
		t.Fatal("no delivery event published")
	}
}

// The query string and path go out exactly as the provider sent them, after
// the forward URL's own, and repeated headers keep each value.
func TestForwarding_RelaysQueryPathAndRepeatedHeadersVerbatim(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	tg := newTarget(t, nil)
	env.createWebhook(t, "wh1", tg.URL+"/my%2Fhooks?sig=a%2Bb&a=0", false)

	req := httptest.NewRequest(http.MethodPost, "/wh1/files/a%2Fb/c%20d?b=x%20y&a=1&a=2&empty=&flag", strings.NewReader("{}"))
	req.Header.Add("X-Multi", "one")
	req.Header.Add("X-Multi", "two, three")
	rec := httptest.NewRecorder()
	env.webhooks.ServeHTTP(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	captured := env.onlyRequest(t, "wh1")
	d := env.awaitDelivery(t, captured.ID)
	got := tg.requests()
	require.Len(t, got, 1)
	assert.Equal(t, "/my%2Fhooks/files/a%2Fb/c%20d", got[0].EscapedPath)
	assert.Equal(t, "sig=a%2Bb&a=0&b=x%20y&a=1&a=2&empty=&flag", got[0].RawQuery)
	assert.Equal(t, []string{"one", "two, three"}, got[0].Header.Values("X-Multi"))
	assert.Equal(t, tg.URL+"/my%2Fhooks/files/a%2Fb/c%20d?sig=a%2Bb&a=0&b=x%20y&a=1&a=2&empty=&flag", d.TargetURL)
}

func TestForwarding_ProviderResponseNotDelayedBySlowTarget(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	release := make(chan struct{})
	reached := make(chan struct{}, 1)
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		reached <- struct{}{}
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	env.createWebhook(t, "wh1", tg.URL, false)

	rec := env.capture(t, http.MethodPost, "/wh1", "{}", nil)
	require.Equal(t, http.StatusAccepted, rec.Code)
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("the target was never reached")
	}
	// The capture answered while the target is still holding the forward.
	assert.Empty(t, env.deliveries(t, env.onlyRequest(t, "wh1").ID))

	close(release)
	d := env.awaitDelivery(t, env.onlyRequest(t, "wh1").ID)
	require.NotNil(t, d.StatusCode)
	assert.Equal(t, http.StatusNoContent, *d.StatusCode)
}

func TestForwarding_RecordsTargetFailures(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()

	release := make(chan struct{})
	stuck := newTarget(t, func(http.ResponseWriter, *http.Request) { <-release })
	// Registered after the target's Close, so it runs first and lets the
	// stuck handler return.
	t.Cleanup(func() { close(release) })
	failing := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	for name, tc := range map[string]struct {
		forwardURL string
		wantStatus int
		wantError  string
	}{
		"500":                {forwardURL: failing.URL, wantStatus: http.StatusInternalServerError},
		"connection refused": {forwardURL: closedURL, wantError: "connection refused"},
		"timeout":            {forwardURL: stuck.URL, wantError: "timed out after 200ms"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := allowLoopback
			cfg.Timeout = 200 * time.Millisecond
			env := newForwardingEnv(t, cfg)
			env.createWebhook(t, "wh1", tc.forwardURL, false)

			rec := env.capture(t, http.MethodPost, "/wh1", "{}", nil)
			require.Equal(t, http.StatusAccepted, rec.Code, "a failing target never affects capture")

			d := env.awaitDelivery(t, env.onlyRequest(t, "wh1").ID)
			if tc.wantStatus != 0 {
				require.NotNil(t, d.StatusCode)
				assert.Equal(t, tc.wantStatus, *d.StatusCode)
				assert.Equal(t, "boom\n", d.ResponseBody)
				assert.Equal(t, models.DeliveryOutcome5xx, d.Outcome)
				assert.Nil(t, d.Error)
				return
			}
			assert.Nil(t, d.StatusCode)
			require.NotNil(t, d.Error)
			assert.Equal(t, tc.wantError, *d.Error)
			assert.Equal(t, models.DeliveryOutcomeError, d.Outcome)
		})
	}
}

func TestForwarding_RecordsRedirectWithoutFollowing(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hook" {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
		}
	})
	env.createWebhook(t, "wh1", tg.URL+"/hook", false)

	env.capture(t, http.MethodPost, "/wh1", "{}", nil)

	d := env.awaitDelivery(t, env.onlyRequest(t, "wh1").ID)
	require.NotNil(t, d.StatusCode)
	assert.Equal(t, http.StatusFound, *d.StatusCode)
	assert.Equal(t, "/elsewhere", d.ResponseHeaders["Location"])
	require.Len(t, tg.requests(), 1, "the redirect isn't followed")
}

func TestForwarding_TruncatesLargeResponseBody(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	large := strings.Repeat("a", models.MaxDeliveryResponseBody+100)
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, large) })
	env.createWebhook(t, "wh1", tg.URL, false)

	env.capture(t, http.MethodPost, "/wh1", "{}", nil)

	d := env.awaitDelivery(t, env.onlyRequest(t, "wh1").ID)
	assert.True(t, d.ResponseBodyTruncated)
	assert.Equal(t, large[:models.MaxDeliveryResponseBody], d.ResponseBody)
}

// Response headers past the cap abort the response, so a target can't make
// a delivery store megabytes of them.
func TestForwarding_RejectsOversizedResponseHeaders(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		chunk := strings.Repeat("h", 4<<10)
		for i := range models.MaxDeliveryResponseHeaders / len(chunk) {
			w.Header().Set(fmt.Sprintf("X-Big-%d", i), chunk)
		}
		w.WriteHeader(http.StatusOK)
	})
	env.createWebhook(t, "wh1", tg.URL, false)

	env.capture(t, http.MethodPost, "/wh1", "{}", nil)

	d := env.awaitDelivery(t, env.onlyRequest(t, "wh1").ID)
	assert.Nil(t, d.StatusCode)
	assert.Empty(t, d.ResponseHeaders)
	require.NotNil(t, d.Error)
	assert.Equal(t, "the target's response headers exceeded 64 KiB", *d.Error)
	assert.Equal(t, models.DeliveryOutcomeError, d.Outcome)
}

// A repeated response header keeps each value, as captured headers do, so
// values containing commas (Set-Cookie) stay unambiguous.
func TestForwarding_KeepsRepeatedResponseHeaders(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "a=1; Expires=Wed, 21 Oct 2026 07:28:00 GMT")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Set("X-Single", "one")
	})
	env.createWebhook(t, "wh1", tg.URL, false)

	env.capture(t, http.MethodPost, "/wh1", "{}", nil)

	d := env.awaitDelivery(t, env.onlyRequest(t, "wh1").ID)
	assert.Equal(t, []string{"a=1; Expires=Wed, 21 Oct 2026 07:28:00 GMT", "b=2"}, models.FieldValues(d.ResponseHeaders["Set-Cookie"]))
	assert.Equal(t, "one", d.ResponseHeaders["X-Single"])
}

func TestForwarding_GuestWebhookDoesNotForward(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	tg := newTarget(t, nil)
	env.createWebhook(t, "guest", tg.URL, true)

	rec := env.capture(t, http.MethodPost, "/guest", "{}", nil)
	require.Equal(t, http.StatusAccepted, rec.Code)

	env.neverDelivered(t, env.onlyRequest(t, "guest").ID)
	assert.Empty(t, tg.requests())
}

// A forward URL leading back to the webhook through a relay (a tunnel, a
// proxy, another hostname for this instance) would loop forever. A request
// carrying the forwarded-request header is still captured, but not forwarded
// again.
func TestForwarding_RelayedBackRequestIsNotForwardedAgain(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	relay := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		// Relays to the webhook endpoint, as a tunnel pointing back would.
		body, _ := io.ReadAll(r.Body)
		req := httptest.NewRequest(r.Method, "/wh1"+strings.TrimPrefix(r.URL.Path, "/relay"), strings.NewReader(string(body)))
		req.Header = r.Header.Clone()
		env.webhooks.ServeHTTP(httptest.NewRecorder(), req)
	})
	env.createWebhook(t, "wh1", relay.URL+"/relay", false)

	env.capture(t, http.MethodPost, "/wh1/orders", "{}", nil)

	require.Eventually(t, func() bool { return len(env.requests(t, "wh1")) == 2 }, 5*time.Second, 5*time.Millisecond)
	captured := env.requests(t, "wh1")
	d := env.awaitDelivery(t, captured[0].ID)
	require.NotNil(t, d.StatusCode, "the original is forwarded")
	assert.Equal(t, captured[0].ID, models.FieldValue(captured[1].Headers[service.RequestIDHeader]), "the relayed copy is captured")
	env.neverDelivered(t, captured[1].ID)
	assert.Len(t, relay.requests(), 1)
	assert.Len(t, env.requests(t, "wh1"), 2)
}

func TestForwarding_QueueFullRecordsDeliveryWithoutBlockingCapture(t *testing.T) {
	cfg := allowLoopback
	cfg.MaxConcurrent = 1
	env := newForwardingEnv(t, cfg)
	release := make(chan struct{})
	reached := make(chan struct{}, 1)
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		reached <- struct{}{}
		<-release
	})
	env.createWebhook(t, "wh1", tg.URL, false)

	env.capture(t, http.MethodPost, "/wh1/first", "{}", nil)
	<-reached
	rec := env.capture(t, http.MethodPost, "/wh1/second", "{}", nil)
	require.Equal(t, http.StatusAccepted, rec.Code)
	close(release)

	captured := env.requests(t, "wh1")
	require.Len(t, captured, 2)
	first, second := env.awaitDelivery(t, captured[0].ID), env.awaitDelivery(t, captured[1].ID)
	assert.NotNil(t, first.StatusCode)
	assert.Nil(t, second.StatusCode)
	require.NotNil(t, second.Error)
	assert.Equal(t, service.ErrMsgQueueFull, *second.Error)
	assert.Equal(t, models.DeliveryOutcomeError, second.Outcome)
	assert.Equal(t, tg.URL+"/second", second.TargetURL)
	assert.Len(t, tg.requests(), 1)
}

// The server shuts the forwarder down on its way out. Captures that are
// still being handled then, even while it drains, record a refused delivery
// instead of starting a forward, and every capture gets exactly one.
func TestForwarding_ShutdownRefusesNewForwards(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	tg := newTarget(t, nil)
	env.createWebhook(t, "wh1", tg.URL, false)

	const captures = 20
	var wg sync.WaitGroup
	for i := range captures {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := env.capture(t, http.MethodPost, fmt.Sprintf("/wh1/%d", i), "{}", nil)
			assert.Equal(t, http.StatusAccepted, rec.Code)
		}()
	}
	require.NoError(t, env.forwarder.Shutdown(context.Background()))
	wg.Wait()

	captured := env.requests(t, "wh1")
	require.Len(t, captured, captures)
	var refused int
	for _, wr := range captured {
		d := env.awaitDelivery(t, wr.ID)
		if d.Error != nil {
			assert.Equal(t, service.ErrMsgShuttingDown, *d.Error)
			assert.Equal(t, tg.URL+wr.Path, d.TargetURL)
			refused++
		}
	}
	assert.Len(t, tg.requests(), captures-refused, "refused captures aren't forwarded")

	rec := env.capture(t, http.MethodPost, "/wh1/late", "{}", nil)
	require.Equal(t, http.StatusAccepted, rec.Code)
	late := env.requests(t, "wh1")[captures]
	d := env.awaitDelivery(t, late.ID)
	require.NotNil(t, d.Error)
	assert.Equal(t, service.ErrMsgShuttingDown, *d.Error)
}

// With the default policy, forwards to loopback - by IP or by a hostname
// resolving to it - are refused at dial time, and the target sees nothing.
func TestForwarding_DefaultPolicyBlocksPrivateDestinations(t *testing.T) {
	tg := newTarget(t, nil)
	_, port, err := net.SplitHostPort(strings.TrimPrefix(tg.URL, "http://"))
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		forwardURL string
		want       string
	}{
		"loopback IP":                  {forwardURL: tg.URL, want: "destination not allowed: 127.0.0.1 is a private or reserved address"},
		"hostname resolving to it":     {forwardURL: "http://localhost:" + port, want: "destination not allowed: localhost resolves to a private or reserved address"},
		"IPv4-mapped IPv6 loopback":    {forwardURL: "http://[::ffff:127.0.0.1]:" + port, want: "destination not allowed: ::ffff:127.0.0.1 is a private or reserved address"},
		"cloud metadata (link-local)":  {forwardURL: "http://169.254.169.254/latest/meta-data", want: "destination not allowed: 169.254.169.254 is a private or reserved address"},
		"private network":              {forwardURL: "http://10.1.2.3:" + port, want: "destination not allowed: 10.1.2.3 is a private or reserved address"},
		"unspecified address":          {forwardURL: "http://0.0.0.0:" + port, want: "destination not allowed: 0.0.0.0 is a private or reserved address"},
		"IPv6 loopback":                {forwardURL: "http://[::1]:" + port, want: "destination not allowed: ::1 is a private or reserved address"},
		"IPv6 unique local (private)":  {forwardURL: "http://[fd00::1]:" + port, want: "destination not allowed: fd00::1 is a private or reserved address"},
		"carrier-grade NAT shared net": {forwardURL: "http://100.64.0.1:" + port, want: "destination not allowed: 100.64.0.1 is a private or reserved address"},
		"6to4 of loopback":             {forwardURL: "http://[2002:7f00:1::1]:" + port, want: "destination not allowed: 2002:7f00:1::1 is a private or reserved address"},
		"Teredo":                       {forwardURL: "http://[2001:0:4136:e378:8000:63bf:3fff:fdd2]:" + port, want: "destination not allowed: 2001:0:4136:e378:8000:63bf:3fff:fdd2 is a private or reserved address"},
	} {
		t.Run(name, func(t *testing.T) {
			env := newForwardingEnv(t, config.Forwarding{Timeout: 2 * time.Second, MaxConcurrent: 4})
			env.createWebhook(t, "wh1", tc.forwardURL, false)

			rec := env.capture(t, http.MethodPost, "/wh1", "{}", nil)
			require.Equal(t, http.StatusAccepted, rec.Code)

			d := env.awaitDelivery(t, env.onlyRequest(t, "wh1").ID)
			assert.Nil(t, d.StatusCode)
			require.NotNil(t, d.Error)
			assert.Equal(t, tc.want, *d.Error)
			assert.Equal(t, models.DeliveryOutcomeBlocked, d.Outcome, "blocked is recorded, not inferred from the message")
		})
	}
	assert.Empty(t, tg.requests())
}

// A request deleted while its forward is in flight can't get a delivery: the
// insert fails on the foreign key, which is logged, not fatal.
func TestForwarding_RequestDeletedMidForward(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	release := make(chan struct{})
	reached := make(chan struct{}, 1)
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		reached <- struct{}{}
		<-release
	})
	env.createWebhook(t, "wh1", tg.URL, false)

	env.capture(t, http.MethodPost, "/wh1", "{}", nil)
	<-reached
	captured := env.onlyRequest(t, "wh1")
	rec := env.postForm(t, "/requests/"+captured.ID+"/delete", nil, "http://example.com/?address=wh1")
	require.Equal(t, http.StatusSeeOther, rec.Code)
	close(release)

	env.neverDelivered(t, captured.ID)
	assert.Empty(t, env.requests(t, "wh1"))
	var n int64
	require.NoError(t, env.db.Model(&models.Delivery{}).Count(&n).Error)
	assert.Zero(t, n)
}

func TestReplay_ToForwardURL(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	var status = http.StatusOK
	var mu sync.Mutex
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.WriteHeader(status)
	})
	env.createWebhook(t, "wh1", tg.URL, false)
	env.capture(t, http.MethodPost, "/wh1/orders?x=1", `{"a": 1}`, map[string]string{"X-Signature": "sig"})
	captured := env.onlyRequest(t, "wh1")
	env.awaitDelivery(t, captured.ID)

	mu.Lock()
	status = http.StatusInternalServerError
	mu.Unlock()
	referer := "http://example.com/requests/" + captured.ID + "?address=wh1"
	rec := env.postForm(t, "/requests/"+captured.ID+"/replay", url.Values{"target": {"forward"}}, referer)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/requests/"+captured.ID+"?address=wh1", rec.Header().Get("Location"))
	assert.Equal(t, &utils.Flash{Kind: utils.FlashSuccess, Message: "Forwarded. Your server answered 500 Internal Server Error."}, flashOf(t, rec))

	got := tg.requests()
	require.Len(t, got, 2)
	assert.Equal(t, got[0].Body, got[1].Body)
	assert.Equal(t, "sig", got[1].Header.Get("X-Signature"))
	assert.Equal(t, "/orders", got[1].Path)

	assert.Len(t, env.requests(t, "wh1"), 1, "replaying to the forward URL captures no copy")
	list := env.deliveries(t, captured.ID)
	require.Len(t, list, 2)
	assert.Equal(t, models.DeliveryTriggerReplay, list[0].Trigger, "newest first")
	require.NotNil(t, list[0].StatusCode)
	assert.Equal(t, http.StatusInternalServerError, *list[0].StatusCode)
	assert.Equal(t, models.DeliveryTriggerAuto, list[1].Trigger)
}

func TestReplay_ToForwardURLNetworkErrorFlashesFailure(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	env.createWebhook(t, "wh1", closed.URL, false)
	env.capture(t, http.MethodPost, "/wh1", "{}", nil)
	captured := env.onlyRequest(t, "wh1")
	env.awaitDelivery(t, captured.ID)

	rec := env.postForm(t, "/requests/"+captured.ID+"/replay", url.Values{"target": {"forward"}}, "http://example.com/?address=wh1")

	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, &utils.Flash{Kind: utils.FlashError, Message: "Forward failed: connection refused."}, flashOf(t, rec))
	assert.Len(t, env.deliveries(t, captured.ID), 2)
}

// The endpoint target, explicit or by default, works as before: the copy is
// captured as a new request, which then forwards like any other.
func TestReplay_ToEndpoint(t *testing.T) {
	for name, form := range map[string]url.Values{
		"default":  nil,
		"explicit": {"target": {"endpoint"}},
	} {
		t.Run(name, func(t *testing.T) {
			// The instance's endpoints are served by the env's webhook
			// router, which needs the endpoint's URL as its domain.
			var webhooks http.Handler
			endpoint := httptest.NewServer(http.StripPrefix("/webhooks", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				webhooks.ServeHTTP(w, r)
			})))
			defer endpoint.Close()
			env := newForwardingEnvAt(t, allowLoopback, endpoint.URL)
			webhooks = env.webhooks
			tg := newTarget(t, nil)
			env.createWebhook(t, "wh1", tg.URL, false)

			env.capture(t, http.MethodPost, "/wh1", "{}", nil)
			captured := env.onlyRequest(t, "wh1")
			env.awaitDelivery(t, captured.ID)

			rec := env.postForm(t, "/requests/"+captured.ID+"/replay", form, "http://example.com/?address=wh1")
			require.Equal(t, http.StatusSeeOther, rec.Code)
			assert.Equal(t, &utils.Flash{Kind: utils.FlashSuccess, Message: "Request replayed. The endpoint answered 202 Accepted."}, flashOf(t, rec))

			all := env.requests(t, "wh1")
			require.Len(t, all, 2, "the replay is captured as a new request")
			assert.Equal(t, models.DeliveryTriggerAuto, env.awaitDelivery(t, all[1].ID).Trigger)
			assert.Len(t, env.deliveries(t, captured.ID), 1, "the original gets no replay delivery")
		})
	}
}

func TestForwarding_DeletesRemoveDeliveries(t *testing.T) {
	for name, del := range map[string]func(t *testing.T, env *forwardingEnv, requestID string) *httptest.ResponseRecorder{
		"delete request": func(t *testing.T, env *forwardingEnv, requestID string) *httptest.ResponseRecorder {
			return env.postForm(t, "/requests/"+requestID+"/delete", nil, "http://example.com/?address=wh1")
		},
		"clear requests": func(t *testing.T, env *forwardingEnv, _ string) *httptest.ResponseRecorder {
			return env.postForm(t, "/delete-requests/wh1", nil, "http://example.com/?address=wh1")
		},
		"delete webhook": func(t *testing.T, env *forwardingEnv, _ string) *httptest.ResponseRecorder {
			return env.postForm(t, "/delete-webhook/wh1", nil, "http://example.com/?address=wh1")
		},
	} {
		t.Run(name, func(t *testing.T) {
			env := newForwardingEnv(t, allowLoopback)
			tg := newTarget(t, nil)
			env.createWebhook(t, "wh1", tg.URL, false)
			env.capture(t, http.MethodPost, "/wh1", "{}", nil)
			captured := env.onlyRequest(t, "wh1")
			env.awaitDelivery(t, captured.ID)

			rec := del(t, env, captured.ID)
			require.Equal(t, http.StatusSeeOther, rec.Code)

			var n int64
			require.NoError(t, env.db.Model(&models.Delivery{}).Count(&n).Error)
			assert.Zero(t, n, fmt.Sprintf("%s removes the deliveries", name))
		})
	}
}

// getPage renders a web page as the signed-in owner.
func (e *forwardingEnv) getPage(t *testing.T, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(e.session)
	rec := httptest.NewRecorder()
	e.web.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestForwarding_PagesShowDeliveries(t *testing.T) {
	env := newForwardingEnv(t, allowLoopback)
	tg := newTarget(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handler", "orders")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "signature mismatch")
	})
	env.createWebhook(t, "wh1", tg.URL, false)
	env.capture(t, http.MethodPost, "/wh1/orders", "{}", nil)
	captured := env.onlyRequest(t, "wh1")
	env.awaitDelivery(t, captured.ID)
	referer := "http://example.com/requests/" + captured.ID + "?address=wh1"
	require.Equal(t, http.StatusSeeOther, env.postForm(t, "/requests/"+captured.ID+"/replay", url.Values{"target": {"forward"}}, referer).Code)
	deliveries := env.deliveries(t, captured.ID)
	require.Len(t, deliveries, 2)

	for _, path := range []string{"/?address=wh1", "/", "/requests/" + captured.ID + "?address=wh1"} {
		t.Run(path, func(t *testing.T) {
			page := env.getPage(t, path)
			replay := strings.Index(page, `id="delivery-`+deliveries[0].ID+`"`)
			auto := strings.Index(page, `id="delivery-`+deliveries[1].ID+`"`)
			require.NotEqual(t, -1, replay, "replay delivery shown")
			assert.Less(t, replay, auto, "newest first")
			assert.Contains(t, page, tg.URL+"/orders")
			assert.Contains(t, page, "signature mismatch")
			assert.Contains(t, page, "X-Handler")
			assert.Contains(t, page, "Replay to forward URL")
		})
	}
	assert.Contains(t, env.getPage(t, "/?address=wh1"), "Last delivery: 500 Internal Server Error", "the row's badge")
}
