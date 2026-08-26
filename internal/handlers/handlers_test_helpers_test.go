package handlers

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
)

// testWebhookRepo is an in-memory implementation of repository.WebhookRepository.
type testWebhookRepo struct {
	mu sync.Mutex

	webhooks map[string]*models.Webhook

	getErr             error
	getByUserErr       error
	getAllErr          error
	getAllByUserErr    error
	insertErr          error
	updateErr          error
	insertRequestErr   error
	deleteErr          error
	getWithRequestsErr error
	cleanPublicErr     error

	insertedRequests []*models.WebhookRequest
}

func newTestWebhookRepo() *testWebhookRepo {
	return &testWebhookRepo{webhooks: make(map[string]*models.Webhook)}
}

func (f *testWebhookRepo) put(w *models.Webhook) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.webhooks[w.ID] = w
}

func (f *testWebhookRepo) Insert(w *models.Webhook) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.put(w)
	return nil
}

func (f *testWebhookRepo) Get(id string) (*models.Webhook, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.webhooks[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return w, nil
}

func (f *testWebhookRepo) GetByUser(id string, userID uint) (*models.Webhook, error) {
	if f.getByUserErr != nil {
		return nil, f.getByUserErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.webhooks[id]
	if !ok || uint(w.UserID) != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return w, nil
}

func (f *testWebhookRepo) GetAll() ([]models.Webhook, error) {
	if f.getAllErr != nil {
		return nil, f.getAllErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.Webhook
	for _, w := range f.webhooks {
		out = append(out, *w)
	}
	return out, nil
}

func (f *testWebhookRepo) GetAllByUser(userID uint) ([]models.Webhook, error) {
	if f.getAllByUserErr != nil {
		return nil, f.getAllByUserErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.Webhook
	for _, w := range f.webhooks {
		if uint(w.UserID) == userID {
			out = append(out, *w)
		}
	}
	return out, nil
}

func (f *testWebhookRepo) Update(w *models.Webhook) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.put(w)
	return nil
}

func (f *testWebhookRepo) InsertRequest(wr *models.WebhookRequest) error {
	if f.insertRequestErr != nil {
		return f.insertRequestErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.insertedRequests = append(f.insertedRequests, wr)
	if w, ok := f.webhooks[wr.WebhookID]; ok {
		w.Requests = append(w.Requests, *wr)
	}
	return nil
}

func (f *testWebhookRepo) Delete(id string, userID uint) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.webhooks[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	if userID != 0 && uint(w.UserID) != userID {
		return gorm.ErrRecordNotFound
	}
	delete(f.webhooks, id)
	return nil
}

func (f *testWebhookRepo) GetWithRequests(id string) (*models.Webhook, error) {
	if f.getWithRequestsErr != nil {
		return nil, f.getWithRequestsErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	w, ok := f.webhooks[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return w, nil
}

func (f *testWebhookRepo) CleanPublic(_ time.Duration) error {
	return f.cleanPublicErr
}

// testWebhookRequestRepo is an in-memory implementation of
// repository.WebhookRequestRepository.
type testWebhookRequestRepo struct {
	mu sync.Mutex

	requests map[string]*models.WebhookRequest

	insertErr          error
	getByIDErr         error
	listErr            error
	deleteByIDErr      error
	deleteByWebhookErr error
}

func newTestWebhookRequestRepo() *testWebhookRequestRepo {
	return &testWebhookRequestRepo{requests: make(map[string]*models.WebhookRequest)}
}

func (f *testWebhookRequestRepo) put(wr *models.WebhookRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests[wr.ID] = wr
}

func (f *testWebhookRequestRepo) Insert(req *models.WebhookRequest) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.put(req)
	return nil
}

func (f *testWebhookRequestRepo) GetByID(id string) (*models.WebhookRequest, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.requests[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return r, nil
}

func (f *testWebhookRequestRepo) ListByWebhook(webhookID string) ([]models.WebhookRequest, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.WebhookRequest
	for _, r := range f.requests {
		if r.WebhookID == webhookID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *testWebhookRequestRepo) DeleteByID(id string) error {
	if f.deleteByIDErr != nil {
		return f.deleteByIDErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.requests, id)
	return nil
}

func (f *testWebhookRequestRepo) DeleteByWebhook(webhookID string) error {
	if f.deleteByWebhookErr != nil {
		return f.deleteByWebhookErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, r := range f.requests {
		if r.WebhookID == webhookID {
			delete(f.requests, id)
		}
	}
	return nil
}

// testUserRepo is an in-memory implementation of repository.UserRepository.
type testUserRepo struct {
	mu sync.Mutex

	byID         map[uint]*models.User
	byEmail      map[string]*models.User
	byResetToken map[string]*models.User
	byAPIKey     map[string]*models.User
	nextID       uint

	createErr          error
	getByIDErr         error
	getByEmailErr      error
	getByResetTokenErr error
	updateErr          error
	getByAPIKeyErr     error
}

func newTestUserRepo() *testUserRepo {
	return &testUserRepo{
		byID:         make(map[uint]*models.User),
		byEmail:      make(map[string]*models.User),
		byResetToken: make(map[string]*models.User),
		byAPIKey:     make(map[string]*models.User),
	}
}

func (f *testUserRepo) index(u *models.User) {
	f.byID[u.ID] = u
	if u.Email != "" {
		f.byEmail[u.Email] = u
	}
	if u.ResetToken != "" {
		f.byResetToken[u.ResetToken] = u
	}
	if u.APIKey != "" {
		f.byAPIKey[u.APIKey] = u
	}
}

// addUser seeds a user directly into the repo, bypassing Create.
func (f *testUserRepo) addUser(u *models.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.ID == 0 {
		f.nextID++
		u.ID = f.nextID
	}
	f.index(u)
}

func (f *testUserRepo) Create(u *models.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	u.ID = f.nextID
	f.index(u)
	return nil
}

func (f *testUserRepo) GetByID(id uint) (*models.User, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (f *testUserRepo) GetByEmail(email string) (*models.User, error) {
	if f.getByEmailErr != nil {
		return nil, f.getByEmailErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byEmail[email]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (f *testUserRepo) GetByResetToken(token string) (*models.User, error) {
	if f.getByResetTokenErr != nil {
		return nil, f.getByResetTokenErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byResetToken[token]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (f *testUserRepo) Update(u *models.User) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.index(u)
	return nil
}

func (f *testUserRepo) GetByAPIKey(key string) (*models.User, error) {
	if f.getByAPIKeyErr != nil {
		return nil, f.getByAPIKeyErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byAPIKey[key]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

// testMetricsRecorder is an in-memory implementation of metrics.Recorder.
type testMetricsRecorder struct {
	mu sync.Mutex

	webhooksCreated int
	signUps         int
	logins          int
	webhookRequests []string
}

func (m *testMetricsRecorder) IncWebhooksCreated() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhooksCreated++
}

func (m *testMetricsRecorder) IncWebhookRequest(webhookID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhookRequests = append(m.webhookRequests, webhookID)
}

func (m *testMetricsRecorder) IncSignUp() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signUps++
}

func (m *testMetricsRecorder) IncLogin() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logins++
}

func newTestLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func newTestAuthService(t *testing.T, repo *testUserRepo) *service.AuthService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return service.NewAuthService(repo, db, "test-secret")
}

const testSessionCookieName = "_webhook_tester_session_id"

// sessionCookieFor logs in the given user against a throwaway request/response
// pair and returns the resulting session cookie so it can be attached to a
// real test request.
func sessionCookieFor(t *testing.T, authSvc *service.AuthService, user *models.User) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	err := authSvc.CreateSession(rec, req, user)
	require.NoError(t, err)
	for _, c := range rec.Result().Cookies() {
		if c.Name == testSessionCookieName {
			return c
		}
	}
	t.Fatal("session cookie not found in response")
	return nil
}

// withURLParam attaches a chi URL param to a request without going through a
// router - useful for exercising edge branches that the real routes can
// never produce (e.g. an empty {id} segment).
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
