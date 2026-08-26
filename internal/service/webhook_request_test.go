package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"webhook-tester/internal/models"
)

type fakeWebhookRequestRepo struct {
	requests map[string]*models.WebhookRequest

	insertErr          error
	getByIDErr         error
	listByWebhookErr   error
	deleteByIDErr      error
	deleteByWebhookErr error

	deletedID        string
	deletedWebhookID string
}

func newFakeWebhookRequestRepo() *fakeWebhookRequestRepo {
	return &fakeWebhookRequestRepo{requests: make(map[string]*models.WebhookRequest)}
}

func (f *fakeWebhookRequestRepo) Insert(req *models.WebhookRequest) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.requests[req.ID] = req
	return nil
}

func (f *fakeWebhookRequestRepo) GetByID(id string) (*models.WebhookRequest, error) {
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	r, ok := f.requests[id]
	if !ok {
		return nil, assert.AnError
	}
	return r, nil
}

func (f *fakeWebhookRequestRepo) ListByWebhook(webhookID string) ([]models.WebhookRequest, error) {
	if f.listByWebhookErr != nil {
		return nil, f.listByWebhookErr
	}
	var out []models.WebhookRequest
	for _, r := range f.requests {
		if r.WebhookID == webhookID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeWebhookRequestRepo) DeleteByID(id string) error {
	f.deletedID = id
	if f.deleteByIDErr != nil {
		return f.deleteByIDErr
	}
	delete(f.requests, id)
	return nil
}

func (f *fakeWebhookRequestRepo) DeleteByWebhook(webhookID string) error {
	f.deletedWebhookID = webhookID
	if f.deleteByWebhookErr != nil {
		return f.deleteByWebhookErr
	}
	for id, r := range f.requests {
		if r.WebhookID == webhookID {
			delete(f.requests, id)
		}
	}
	return nil
}

func TestWebhookRequestService_Record(t *testing.T) {
	repo := newFakeWebhookRequestRepo()
	svc := NewWebhookRequestService(repo)
	rq := &models.WebhookRequest{ID: "r1"}

	err := svc.Record(rq)
	require.NoError(t, err)
	assert.Equal(t, rq, repo.requests["r1"])

	repo.insertErr = assert.AnError
	err = svc.Record(rq)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookRequestService_Get(t *testing.T) {
	repo := newFakeWebhookRequestRepo()
	svc := NewWebhookRequestService(repo)
	repo.requests["r1"] = &models.WebhookRequest{ID: "r1"}

	r, err := svc.Get("r1")
	require.NoError(t, err)
	assert.Equal(t, "r1", r.ID)

	_, err = svc.Get("missing")
	assert.Error(t, err)
}

func TestWebhookRequestService_List(t *testing.T) {
	repo := newFakeWebhookRequestRepo()
	svc := NewWebhookRequestService(repo)
	repo.requests["r1"] = &models.WebhookRequest{ID: "r1", WebhookID: "wh1"}
	repo.requests["r2"] = &models.WebhookRequest{ID: "r2", WebhookID: "wh2"}

	list, err := svc.List("wh1")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	repo.listByWebhookErr = assert.AnError
	_, err = svc.List("wh1")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookRequestService_Delete(t *testing.T) {
	repo := newFakeWebhookRequestRepo()
	svc := NewWebhookRequestService(repo)
	repo.requests["r1"] = &models.WebhookRequest{ID: "r1"}

	err := svc.Delete("r1")
	require.NoError(t, err)
	assert.Equal(t, "r1", repo.deletedID)

	repo.deleteByIDErr = assert.AnError
	err = svc.Delete("r1")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookRequestService_DeleteAll(t *testing.T) {
	repo := newFakeWebhookRequestRepo()
	svc := NewWebhookRequestService(repo)
	repo.requests["r1"] = &models.WebhookRequest{ID: "r1", WebhookID: "wh1"}

	err := svc.DeleteAll("wh1")
	require.NoError(t, err)
	assert.Equal(t, "wh1", repo.deletedWebhookID)
	assert.NotContains(t, repo.requests, "r1")

	repo.deleteByWebhookErr = assert.AnError
	err = svc.DeleteAll("wh1")
	assert.ErrorIs(t, err, assert.AnError)
}
