package routers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"webhook-tester/config"
	webhookdb "webhook-tester/internal/db"
	"webhook-tester/internal/dtos"
	appMetrics "webhook-tester/internal/metrics"
	"webhook-tester/internal/routers"
	"webhook-tester/internal/service"
	"webhook-tester/internal/store"
)

func newAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Foreign keys on, as Postgres enforces them in production. One
	// connection, since each new connection to ":memory:" would open a
	// fresh, empty database - and forwards write from other goroutines.
	db, err := gorm.Open(sqlite.Open(":memory:?_foreign_keys=on"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	webhookdb.AutoMigrate(db)
	return db
}

// newTestForwarder returns a forwarder recording through webhookSvc. It
// waits for its in-flight forwards when the test ends, before the DB closes.
func newTestForwarder(t *testing.T, webhookSvc *service.WebhookService, cfg config.Forwarding) *service.Forwarder {
	t.Helper()
	f := service.NewForwarder(cfg, webhookSvc, &appMetrics.PrometheusRecorder{}, testLogger())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		require.NoError(t, f.Shutdown(ctx), "in-flight forwards didn't finish")
	})
	return f
}

// testDomain is the DOMAIN the routers under test are served at.
const testDomain = "https://tester.example.com"

// testResolver resolves the hosts it lists and fails every other lookup,
// as for an unknown host, so tests never query real DNS.
type testResolver map[string][]netip.Addr

func (r testResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	if addrs, ok := r[host]; ok {
		return addrs, nil
	}
	return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
}

// privateHost is a hostname testForwardPolicy resolves to a private address.
const privateHost = "db.internal.example.com"

// testForwardPolicy is the default forwarding policy (private networks not
// allowed) over testResolver.
var testForwardPolicy = service.ForwardPolicy{
	Resolver: testResolver{privateHost: {netip.MustParseAddr("10.0.0.7")}},
}

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

// setupAPIRouter builds a real API router backed by a fresh sqlite DB and
// returns the router plus the API key of a freshly-registered user.
func setupAPIRouter(t *testing.T) (http.Handler, string) {
	t.Helper()
	return setupAPIRouterWith(t, testForwardPolicy)
}

// setupAPIRouterWith is setupAPIRouter for an instance with the given
// forwarding policy.
func setupAPIRouterWith(t *testing.T, policy service.ForwardPolicy) (http.Handler, string) {
	t.Helper()

	db := newAPITestDB(t)
	logger := testLogger()

	userRepo := store.NewGormUserRepo(db, logger)
	webhookRepo := store.NewGormWebookRepo(db, logger)

	authSvc := service.NewAuthService(userRepo, db, "test-auth-secret")
	webhookSvc := service.NewWebhookService(webhookRepo, store.NewGormDeliveryRepo(db, logger), testDomain, policy)

	user, err := authSvc.Register("api-user@example.com", "Passw0rd!", "API User")
	require.NoError(t, err)

	metricsRec := &appMetrics.PrometheusRecorder{}

	r := routers.NewApiRouter(webhookSvc, authSvc, logger, metricsRec)

	return r, user.APIKey
}

func TestNewApiRouter_RequiresAPIKey(t *testing.T) {
	r, _ := setupAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestNewApiRouter_PostWebhooksWithoutKey(t *testing.T) {
	r, _ := setupAPIRouter(t)

	body, _ := json.Marshal(dtos.CreateWebhookRequest{Title: "no key"})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestNewApiRouter_CRUDWithValidKey(t *testing.T) {
	r, apiKey := setupAPIRouter(t)

	// Create
	createBody, _ := json.Marshal(dtos.CreateWebhookRequest{
		Title:        "my webhook",
		ContentType:  "application/json",
		Payload:      `{"ok":true}`,
		ResponseCode: 200,
	})
	req := httptest.NewRequest(http.MethodPost, "/webhooks/", bytes.NewReader(createBody))
	req.Header.Set("X-API-Key", apiKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)

	// List
	req = httptest.NewRequest(http.MethodGet, "/webhooks/", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Get by ID
	req = httptest.NewRequest(http.MethodGet, "/webhooks/"+created.ID+"/", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Get by ID without key -> unauthorized
	req = httptest.NewRequest(http.MethodGet, "/webhooks/"+created.ID+"/", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// Update
	title := "updated title"
	updateBody, _ := json.Marshal(dtos.UpdateWebhookRequest{Title: &title})
	req = httptest.NewRequest(http.MethodPatch, "/webhooks/"+created.ID+"/", bytes.NewReader(updateBody))
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var updated dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	require.Equal(t, "updated title", updated.Title)

	// PUT is an alias of PATCH
	req = httptest.NewRequest(http.MethodPut, "/webhooks/"+created.ID+"/", bytes.NewReader([]byte(`{"title":"via put"}`)))
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &updated))
	require.Equal(t, "via put", updated.Title)

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/webhooks/"+created.ID+"/", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)

	// Delete again -> not found
	req = httptest.NewRequest(http.MethodDelete, "/webhooks/"+created.ID+"/", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNewApiRouter_JSONErrors(t *testing.T) {
	r, apiKey := setupAPIRouter(t)

	cases := []struct {
		method, path, key string
		want              int
	}{
		{http.MethodGet, "/webhooks/", "", http.StatusUnauthorized},
		{http.MethodGet, "/webhooks/", "bogus", http.StatusUnauthorized},
		{http.MethodGet, "/nope", apiKey, http.StatusNotFound},
		{http.MethodPatch, "/webhooks/", apiKey, http.StatusMethodNotAllowed},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		if c.key != "" {
			req.Header.Set("X-API-Key", c.key)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		require.Equal(t, c.want, rec.Code, c.path)
		require.Contains(t, rec.Header().Get("Content-Type"), "application/json", c.path)
		var body dtos.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), c.path)
		require.NotEmpty(t, body.Error, c.path)
	}
}

func TestNewApiRouter_ForwardURLRoundTrip(t *testing.T) {
	r, apiKey := setupAPIRouter(t)
	do := func(method, path, body string) (int, dtos.Webhook, string) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("X-API-Key", apiKey)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		var wh dtos.Webhook
		if rec.Code < 300 && method != http.MethodDelete {
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &wh), rec.Body.String())
		}
		return rec.Code, wh, rec.Body.String()
	}
	getForwardURL := func(id string) *string {
		t.Helper()
		code, wh, _ := do(http.MethodGet, "/webhooks/"+id+"/", "")
		require.Equal(t, http.StatusOK, code)
		return wh.ForwardURL
	}

	// Create with a forward URL; surrounding whitespace is trimmed.
	code, created, _ := do(http.MethodPost, "/webhooks/", `{"title":"fwd","forward_url":"  https://api.example.com/hooks  "}`)
	require.Equal(t, http.StatusCreated, code)
	require.NotNil(t, created.ForwardURL)
	require.Equal(t, "https://api.example.com/hooks", *created.ForwardURL)
	require.Equal(t, "https://api.example.com/hooks", *getForwardURL(created.ID))

	// List includes it.
	req := httptest.NewRequest(http.MethodGet, "/webhooks/", nil)
	req.Header.Set("X-API-Key", apiKey)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var listed []dtos.Webhook
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.Len(t, listed, 1)
	require.Equal(t, "https://api.example.com/hooks", *listed[0].ForwardURL)

	path := "/webhooks/" + created.ID + "/"

	// An update that leaves forward_url out keeps it.
	code, _, _ = do(http.MethodPatch, path, `{"title":"renamed"}`)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "https://api.example.com/hooks", *getForwardURL(created.ID))

	// Change it.
	code, updated, _ := do(http.MethodPatch, path, `{"forward_url":"https://abc.ngrok-free.app/stripe"}`)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "https://abc.ngrok-free.app/stripe", *updated.ForwardURL)

	// Invalid values are rejected and change nothing.
	for bad, wantErr := range map[string]string{
		`"ftp://files.example.com"`: "absolute http or https URL",
		`"/relative"`:               "absolute http or https URL",
		`"https://tester.example.com/webhooks/` + created.ID + `"`: "own webhook endpoints",
		`42`:            "expected a string or null, got a number",
		`{"url":"x"}`:   "expected a string or null, got an object",
		`["https://x"]`: "expected a string or null, got an array",
	} {
		code, _, body := do(http.MethodPatch, path, `{"forward_url":`+bad+`}`)
		require.Equal(t, http.StatusBadRequest, code, bad)
		require.Contains(t, body, wantErr, bad)
		require.Equal(t, "https://abc.ngrok-free.app/stripe", *getForwardURL(created.ID), bad)
	}

	// An explicit null clears it, and so does an empty string.
	code, updated, _ = do(http.MethodPatch, path, `{"forward_url":null}`)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, updated.ForwardURL)
	require.Nil(t, getForwardURL(created.ID))

	code, _, _ = do(http.MethodPatch, path, `{"forward_url":"https://api.example.com/hooks"}`)
	require.Equal(t, http.StatusOK, code)
	code, updated, _ = do(http.MethodPut, path, `{"forward_url":""}`)
	require.Equal(t, http.StatusOK, code)
	require.Nil(t, updated.ForwardURL)

	// A webhook created without one reports null.
	code, plain, body := do(http.MethodPost, "/webhooks/", `{"title":"plain"}`)
	require.Equal(t, http.StatusCreated, code)
	require.Nil(t, plain.ForwardURL)
	require.Contains(t, body, `"forward_url":null`)

	// Create rejects a forward URL that loops back to this instance.
	code, _, body = do(http.MethodPost, "/webhooks/", `{"title":"loop","forward_url":"https://tester.example.com/webhooks/abc"}`)
	require.Equal(t, http.StatusBadRequest, code)
	require.Contains(t, body, "own webhook endpoints")
}

// With private networks not allowed, create, update and PATCH reject a
// forward URL the forwarder could never reach. A self-hosted instance that
// allows them accepts it.
func TestNewApiRouter_PrivateForwardURL(t *testing.T) {
	privateURLs := []string{
		"http://localhost:8080/hooks",
		"http://app.localhost/hooks",
		"http://127.0.0.1:3000/hooks",
		"http://[::1]/hooks",
		"http://169.254.169.254/latest/meta-data",
		"http://" + privateHost + "/hooks",
	}
	for _, allow := range []bool{false, true} {
		r, apiKey := setupAPIRouterWith(t, service.ForwardPolicy{AllowPrivateNetworks: allow, Resolver: testForwardPolicy.Resolver})
		do := func(method, path, body string) (int, string) {
			t.Helper()
			req := httptest.NewRequest(method, path, strings.NewReader(body))
			req.Header.Set("X-API-Key", apiKey)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			return rec.Code, rec.Body.String()
		}
		code, body := do(http.MethodPost, "/webhooks/", `{"title":"plain"}`)
		require.Equal(t, http.StatusCreated, code)
		var created dtos.Webhook
		require.NoError(t, json.Unmarshal([]byte(body), &created))
		path := "/webhooks/" + created.ID + "/"

		for _, forwardURL := range privateURLs {
			requests := map[string]struct{ method, path, body string }{
				"create": {http.MethodPost, "/webhooks/", `{"title":"fwd","forward_url":"` + forwardURL + `"}`},
				"update": {http.MethodPut, path, `{"title":"plain","forward_url":"` + forwardURL + `"}`},
				"patch":  {http.MethodPatch, path, `{"forward_url":"` + forwardURL + `"}`},
			}
			for name, req := range requests {
				code, body := do(req.method, req.path, req.body)
				if allow {
					require.Less(t, code, 300, "%s %s: %s", name, forwardURL, body)
					continue
				}
				require.Equal(t, http.StatusBadRequest, code, "%s %s", name, forwardURL)
				require.Contains(t, body, "forward URL points to a private or local address", name)
				require.Contains(t, body, "use a public tunnel URL (ngrok, cloudflared)", name)
			}
		}
		if !allow {
			code, body := do(http.MethodGet, path, "")
			require.Equal(t, http.StatusOK, code)
			require.Contains(t, body, `"forward_url":null`, "rejected updates change nothing")
		}
	}
}

func TestNewApiRouter_InvalidKey(t *testing.T) {
	r, _ := setupAPIRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/webhooks/", nil)
	req.Header.Set("X-API-Key", "not-a-real-key")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
