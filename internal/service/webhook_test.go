package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gorm.io/gorm"

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
	cleanPublicIDs     []string
	countRequestsErr   error
	requestsAfterErr   error
	assignOwnerErr     error

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
	if w, ok := f.webhooks[wr.WebhookID]; ok {
		w.Requests = append(w.Requests, *wr)
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

func (f *fakeWebhookRepo) CleanPublic(d time.Duration) ([]string, error) {
	f.cleanPublicDuration = d
	if f.cleanPublicErr != nil {
		return nil, f.cleanPublicErr
	}
	return f.cleanPublicIDs, nil
}

func (f *fakeWebhookRepo) GetRequestsAfter(webhookID string, after models.RequestCursor) ([]models.WebhookRequest, error) {
	if f.requestsAfterErr != nil {
		return nil, f.requestsAfterErr
	}
	var out []models.WebhookRequest
	if w, ok := f.webhooks[webhookID]; ok {
		for _, wr := range w.Requests {
			if after.Before(models.CursorAt(wr)) {
				out = append(out, wr)
			}
		}
	}
	return out, nil
}

func (f *fakeWebhookRepo) AssignOwner(id string, userID uint) error {
	if f.assignOwnerErr != nil {
		return f.assignOwnerErr
	}
	w, ok := f.webhooks[id]
	if !ok || w.UserID != 0 {
		return gorm.ErrRecordNotFound
	}
	w.UserID = int(userID)
	return nil
}

func (f *fakeWebhookRepo) CountRequests(webhookID string) (int64, error) {
	if f.countRequestsErr != nil {
		return 0, f.countRequestsErr
	}
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

// receive returns the next event on sub, failing the test if none arrives.
func receive(t *testing.T, sub *Subscription) Event {
	t.Helper()
	select {
	case evt, ok := <-sub.Events:
		require.True(t, ok, "subscription closed unexpectedly")
		return evt
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for an event")
		return Event{}
	}
}

// requireClosed fails the test unless sub's Events channel is closed.
func requireClosed(t *testing.T, sub *Subscription) {
	t.Helper()
	select {
	case _, ok := <-sub.Events:
		require.False(t, ok, "expected the subscription to be closed")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the subscription to close")
	}
}

func TestWebhookService_RecordRequest_StoresStampsAndPublishes(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["abc"] = &models.Webhook{ID: "abc"}
	sub := svc.Subscribe("abc")
	defer sub.Close()
	other := svc.Subscribe("other")
	defer other.Close()

	before := time.Now().UTC()
	wr := &models.WebhookRequest{ID: "r1", WebhookID: "abc"}
	require.NoError(t, svc.RecordRequest(wr))

	assert.Equal(t, wr, repo.insertedRequest)
	assert.False(t, wr.ReceivedAt.Before(before.Truncate(time.Microsecond)))
	assert.Equal(t, wr.ReceivedAt, wr.ReceivedAt.Truncate(time.Microsecond), "stamped at DB precision")

	evt := receive(t, sub)
	assert.Equal(t, EventRequestCaptured, evt.Kind)
	assert.Equal(t, "r1", evt.Request.ID)
	require.NotNil(t, evt.Count)
	assert.Equal(t, int64(1), *evt.Count)
	assert.Empty(t, other.Events, "other webhooks' subscribers get nothing")
}

func TestWebhookService_PublishDelivery(t *testing.T) {
	svc := NewWebhookService(newFakeWebhookRepo())
	sub := svc.Subscribe("abc")
	defer sub.Close()
	other := svc.Subscribe("other")
	defer other.Close()

	svc.PublishDelivery(models.Delivery{ID: "d1", RequestID: "r1", WebhookID: "abc"})

	evt := receive(t, sub)
	assert.Equal(t, EventDeliveryRecorded, evt.Kind)
	assert.Equal(t, "d1", evt.Delivery.ID)
	assert.Equal(t, "r1", evt.Delivery.RequestID)
	assert.Empty(t, other.Events, "other webhooks' subscribers get nothing")
}

func TestWebhookService_RecordRequest_CountErrorStillPublishes(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.countRequestsErr = assert.AnError
	sub := svc.Subscribe("abc")
	defer sub.Close()

	require.NoError(t, svc.RecordRequest(&models.WebhookRequest{ID: "r1", WebhookID: "abc"}))

	evt := receive(t, sub)
	assert.Equal(t, "r1", evt.Request.ID)
	assert.Nil(t, evt.Count)
}

func TestWebhookService_RecordRequest_InsertErrorPublishesNothing(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.insertRequestErr = assert.AnError
	sub := svc.Subscribe("abc")
	defer sub.Close()

	err := svc.RecordRequest(&models.WebhookRequest{ID: "r1", WebhookID: "abc"})
	assert.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, sub.Events)
}

// Concurrent captures for one webhook publish in ReceivedAt order, which is
// what makes the last delivered event a gap-free resume cursor.
func TestWebhookService_RecordRequest_PublishesInCursorOrder(t *testing.T) {
	svc := NewWebhookService(newFakeWebhookRepo())
	sub := svc.Subscribe("abc")
	defer sub.Close()

	const n = subscriptionBuffer
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, svc.RecordRequest(&models.WebhookRequest{ID: fmt.Sprint(i), WebhookID: "abc"}))
		}()
	}
	wg.Wait()

	var prev models.RequestCursor
	for i := 0; i < n; i++ {
		cur := models.CursorAt(receive(t, sub).Request)
		assert.True(t, prev.Before(cur), "event %d out of order", i)
		prev = cur
	}
}

func TestWebhookService_RecordRequest_ReleasesWebhookLocks(t *testing.T) {
	svc := NewWebhookService(newFakeWebhookRepo())
	for i := 0; i < 3; i++ {
		require.NoError(t, svc.RecordRequest(&models.WebhookRequest{ID: fmt.Sprint(i), WebhookID: fmt.Sprint("wh", i)}))
	}
	assert.Empty(t, svc.broker.locks)
}

// A subscriber that stops reading is evicted rather than blocking capture;
// it resumes from its cursor on reconnect.
func TestWebhookService_Subscribe_EvictsSlowSubscriber(t *testing.T) {
	svc := NewWebhookService(newFakeWebhookRepo())
	slow := svc.Subscribe("abc")
	defer slow.Close()

	for i := 0; i <= subscriptionBuffer; i++ {
		require.NoError(t, svc.RecordRequest(&models.WebhookRequest{ID: fmt.Sprint(i), WebhookID: "abc"}))
	}

	for i := 0; i < subscriptionBuffer; i++ {
		receive(t, slow)
	}
	requireClosed(t, slow)
	assert.Empty(t, svc.broker.subs)
}

func TestWebhookService_Subscription_CloseIsIdempotent(t *testing.T) {
	svc := NewWebhookService(newFakeWebhookRepo())
	sub := svc.Subscribe("abc")
	sub.Close()
	sub.Close()
	requireClosed(t, sub)
	assert.Empty(t, svc.broker.subs)
}

func TestWebhookService_GetRequestsAfter(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	t0 := time.Now().UTC()
	repo.webhooks["abc"] = &models.Webhook{ID: "abc", Requests: []models.WebhookRequest{
		{ID: "r1", ReceivedAt: t0},
		{ID: "r2", ReceivedAt: t0.Add(time.Second)},
	}}

	got, err := svc.GetRequestsAfter("abc", models.RequestCursor{ReceivedAt: t0, ID: "r1"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "r2", got[0].ID)
}

func TestWebhookService_GetAccessibleWebhook(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["public"] = &models.Webhook{ID: "public"}
	repo.webhooks["owned"] = &models.Webhook{ID: "owned", UserID: 7}

	for _, tc := range []struct {
		id     string
		userID uint
		ok     bool
	}{
		{"public", 0, true},
		{"public", 7, true},
		{"owned", 7, true},
		{"owned", 0, false},
		{"owned", 8, false},
	} {
		wh, err := svc.GetAccessibleWebhook(tc.id, tc.userID)
		whr, errR := svc.GetAccessibleWebhookWithRequests(tc.id, tc.userID)
		if tc.ok {
			require.NoError(t, err, "%+v", tc)
			require.NoError(t, errR, "%+v", tc)
			assert.Equal(t, tc.id, wh.ID)
			assert.Equal(t, tc.id, whr.ID)
		} else {
			assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "%+v", tc)
			assert.ErrorIs(t, errR, gorm.ErrRecordNotFound, "%+v", tc)
		}
	}

	_, err := svc.GetAccessibleWebhook("missing", 0)
	assert.Error(t, err)
	_, err = svc.GetAccessibleWebhookWithRequests("missing", 0)
	assert.Error(t, err)
}

func TestWebhookService_DeleteWebhook(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["abc"] = &models.Webhook{ID: "abc"}

	sub := svc.Subscribe("abc")

	repo.deleteErr = assert.AnError
	err := svc.DeleteWebhook("abc", 1)
	assert.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, sub.Events, "a failed delete keeps subscriptions open")

	repo.deleteErr = nil
	err = svc.DeleteWebhook("abc", 1)
	require.NoError(t, err)
	assert.Equal(t, "abc", repo.deletedID)
	assert.Equal(t, uint(1), repo.deletedUserID)
	requireClosed(t, sub)
}

func TestWebhookService_CountRequests(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["abc"] = &models.Webhook{
		ID:       "abc",
		Requests: []models.WebhookRequest{{ID: "r1"}, {ID: "r2"}},
	}

	count, err := svc.CountRequests("abc")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	repo.countRequestsErr = assert.AnError
	_, err = svc.CountRequests("abc")
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookService_CleanPublicWebhooks(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)

	deleted := svc.Subscribe("old")
	kept := svc.Subscribe("new")
	defer kept.Close()
	repo.cleanPublicIDs = []string{"old"}

	err := svc.CleanPublicWebhooks(time.Hour)
	require.NoError(t, err)
	assert.Equal(t, time.Hour, repo.cleanPublicDuration)
	requireClosed(t, deleted)
	assert.Empty(t, kept.Events)

	repo.cleanPublicErr = assert.AnError
	err = svc.CleanPublicWebhooks(time.Hour)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestWebhookService_ClaimGuestWebhook(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["guest"] = &models.Webhook{ID: "guest"}
	repo.webhooks["owned"] = &models.Webhook{ID: "owned", UserID: 3}

	require.NoError(t, svc.ClaimGuestWebhook("guest", 7))
	assert.Equal(t, 7, repo.webhooks["guest"].UserID)

	assert.ErrorIs(t, svc.ClaimGuestWebhook("owned", 7), gorm.ErrRecordNotFound, "can't take another user's webhook")
	assert.Equal(t, 3, repo.webhooks["owned"].UserID)
	assert.ErrorIs(t, svc.ClaimGuestWebhook("missing", 7), gorm.ErrRecordNotFound)
}

func TestWebhookService_GetUserWebhookWithRequests(t *testing.T) {
	repo := newFakeWebhookRepo()
	svc := NewWebhookService(repo)
	repo.webhooks["mine"] = &models.Webhook{ID: "mine", UserID: 7, Requests: []models.WebhookRequest{{ID: "r1"}}}
	repo.webhooks["guest"] = &models.Webhook{ID: "guest"}

	wh, err := svc.GetUserWebhookWithRequests("mine", 7)
	require.NoError(t, err)
	assert.Len(t, wh.Requests, 1)

	_, err = svc.GetUserWebhookWithRequests("guest", 7)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "public webhooks aren't the user's")
	_, err = svc.GetUserWebhookWithRequests("mine", 8)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
