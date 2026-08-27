package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"webhook-tester/internal/models"
)

type fakeWebhookRepo struct {
	webhooks map[string]*models.Webhook

	insertErr          error
	getErr             error
	getByUserErr       error
	getAllErr          error
	getAllByUserErr    error
	updateErr          error
	insertRequestErr   error
	deleteErr          error
	getWithRequestsErr error
	cleanPublicErr     error

	getAllCalled        bool
	getAllByUserCalled  bool
	getAllByUserArg     uint
	insertedRequest     *models.WebhookRequest
	deletedID           string
	deletedUserID       uint
	updatedWebhook      *models.Webhook
	getWithRequestsID   string
	cleanPublicDuration time.Duration
}

func newFakeWebhookRepo() *fakeWebhookRepo {
	return &fakeWebhookRepo{webhooks: make(map[string]*models.Webhook)}
}

func (f *fakeWebhookRepo) Insert(webhook *models.Webhook) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.webhooks[webhook.ID] = webhook
	return nil
}

func (f *fakeWebhookRepo) Get(id string) (*models.Webhook, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	w, ok := f.webhooks[id]
	if !ok {
		return nil, assert.AnError
	}
	return w, nil
}

func (f *fakeWebhookRepo) GetByUser(id string, userID uint) (*models.Webhook, error) {
	if f.getByUserErr != nil {
		return nil, f.getByUserErr
	}
	w, ok := f.webhooks[id]
	if !ok || w.UserID != int(userID) {
		return nil, assert.AnError
	}
	return w, nil
}

func (f *fakeWebhookRepo) GetAll() ([]models.Webhook, error) {
	f.getAllCalled = true
	if f.getAllErr != nil {
		return nil, f.getAllErr
	}
	var out []models.Webhook
	for _, w := range f.webhooks {
		out = append(out, *w)
	}
	return out, nil
}

func (f *fakeWebhookRepo) GetAllByUser(userID uint) ([]models.Webhook, error) {
	f.getAllByUserCalled = true
	f.getAllByUserArg = userID
	if f.getAllByUserErr != nil {
		return nil, f.getAllByUserErr
	}
	var out []models.Webhook
	for _, w := range f.webhooks {
		if w.UserID == int(userID) {
			out = append(out, *w)
		}
	}
	return out, nil
}

func (f *fakeWebhookRepo) Update(webhook *models.Webhook) error {
	f.updatedWebhook = webhook
	if f.updateErr != nil {
		return f.updateErr
	}
	f.webhooks[webhook.ID] = webhook
	return nil
}

func (f *fakeWebhookRepo) InsertRequest(wr *models.WebhookRequest) error {
	f.insertedRequest = wr
	if f.insertRequestErr != nil {
		return f.insertRequestErr
	}
	return nil
}

func (f *fakeWebhookRepo) Delete(id string, userID uint) error {
	f.deletedID = id
	f.deletedUserID = userID
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.webhooks, id)
	return nil
}

func (f *fakeWebhookRepo) GetWithRequests(id string) (*models.Webhook, error) {
	f.getWithRequestsID = id
	if f.getWithRequestsErr != nil {
		return nil, f.getWithRequestsErr
	}
	w, ok := f.webhooks[id]
	if !ok {
		return nil, assert.AnError
	}
	return w, nil
}

func (f *fakeWebhookRepo) CleanPublic(d time.Duration) error {
	f.cleanPublicDuration = d
	return f.cleanPublicErr
}

func (f *fakeWebhookRepo) CountRequests(webhookID string) (int64, error) {
	w, ok := f.webhooks[webhookID]
	if !ok {
		return 0, nil
	}
	return int64(len(w.Requests)), nil
}

func TestWebhookService_CreateWebhook(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)

	w := &models.Webhook{ID: "abc"}
	err := svc.CreateWebhook(w)
	require.NoError(t, err)
	assert.Equal(t, w, repo.webhooks["abc"])

	repo.insertErr = assert.AnError
	err = svc.CreateWebhook(w)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookService_GetWebhook(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["abc"] = &models.Webhook{ID: "abc"}

	w, err := svc.GetWebhook("abc")
	require.NoError(t, err)
	assert.Equal(t, "abc", w.ID)

	_, err = svc.GetWebhook("missing")
	assert.Error(t, err)
}

func TestWebhookService_GetUserWebhook(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["abc"] = &models.Webhook{ID: "abc", UserID: 5}

	w, err := svc.GetUserWebhook("abc", 5)
	require.NoError(t, err)
	assert.Equal(t, "abc", w.ID)

	_, err = svc.GetUserWebhook("abc", 9)
	assert.Error(t, err)

	repo.getByUserErr = assert.AnError
	_, err = svc.GetUserWebhook("abc", 5)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookService_ListWebhooks_Public(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["abc"] = &models.Webhook{ID: "abc"}

	list, err := svc.ListWebhooks(0)
	require.NoError(t, err)
	assert.True(t, repo.getAllCalled)
	assert.False(t, repo.getAllByUserCalled)
	assert.Len(t, list, 1)
}

func TestWebhookService_ListWebhooks_ByUser(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["abc"] = &models.Webhook{ID: "abc", UserID: 7}

	list, err := svc.ListWebhooks(7)
	require.NoError(t, err)
	assert.False(t, repo.getAllCalled)
	assert.True(t, repo.getAllByUserCalled)
	assert.Equal(t, uint(7), repo.getAllByUserArg)
	assert.Len(t, list, 1)
}

func TestWebhookService_ListWebhooks_Errors(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)

	repo.getAllErr = assert.AnError
	_, err := svc.ListWebhooks(0)
	assert.ErrorIs(t, err, assert.AnError)

	repo.getAllByUserErr = assert.AnError
	_, err = svc.ListWebhooks(3)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookService_UpdateWebhook(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	w := &models.Webhook{ID: "abc"}

	err := svc.UpdateWebhook(w)
	require.NoError(t, err)
	assert.Equal(t, w, repo.updatedWebhook)

	repo.updateErr = assert.AnError
	err = svc.UpdateWebhook(w)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookService_CreateRequest(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	wr := &models.WebhookRequest{ID: "r1"}

	err := svc.CreateRequest(wr)
	require.NoError(t, err)
	assert.Equal(t, wr, repo.insertedRequest)

	repo.insertRequestErr = assert.AnError
	err = svc.CreateRequest(wr)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookService_DeleteWebhook(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["abc"] = &models.Webhook{ID: "abc"}

	err := svc.DeleteWebhook("abc", 1)
	require.NoError(t, err)
	assert.Equal(t, "abc", repo.deletedID)
	assert.Equal(t, uint(1), repo.deletedUserID)

	repo.deleteErr = assert.AnError
	err = svc.DeleteWebhook("abc", 1)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookService_GetWebhookWithRequests(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["abc"] = &models.Webhook{ID: "abc"}

	w, err := svc.GetWebhookWithRequests("abc")
	require.NoError(t, err)
	assert.Equal(t, "abc", w.ID)

	_, err = svc.GetWebhookWithRequests("missing")
	assert.Error(t, err)
}

func TestWebhookService_CleanPublicWebhooks(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)

	err := svc.CleanPublicWebhooks(time.Hour)
	require.NoError(t, err)
	assert.Equal(t, time.Hour, repo.cleanPublicDuration)

	repo.cleanPublicErr = assert.AnError
	err = svc.CleanPublicWebhooks(time.Hour)
	assert.ErrorIs(t, err, assert.AnError)
}
