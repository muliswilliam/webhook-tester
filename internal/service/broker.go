package service

import (
	"sync"
	"time"

	"webhook-tester/internal/models"
)

// RequestEvent is published to a webhook's subscribers each time a request is
// captured. Count is the webhook's request total right after the insert, or
// nil if counting failed.
type RequestEvent struct {
	Request models.WebhookRequest
	Count   *int64
}

// subscriptionBuffer bounds how far a subscriber may fall behind before it is
// evicted. Evicted subscribers reconnect and replay what they missed from the
// DB, so a small buffer costs a reconnect, never a lost request.
const subscriptionBuffer = 32

// Subscription receives a webhook's RequestEvents until Events is closed,
// which happens when the subscriber falls too far behind, the webhook is
// deleted, or Close is called.
type Subscription struct {
	Events <-chan RequestEvent

	events    chan RequestEvent
	webhookID string
	broker    *broker
}

// Close unsubscribes. It is safe to call more than once.
func (s *Subscription) Close() {
	s.broker.remove(s.webhookID, s)
}

// broker fans captured requests out to live subscribers, and serializes
// captures per webhook so each webhook's events are published in insert
// order.
type broker struct {
	mu   sync.Mutex
	subs map[string]map[*Subscription]struct{}

	locksMu sync.Mutex
	locks   map[string]*webhookLock
}

type webhookLock struct {
	sync.Mutex
	refs      int
	lastStamp time.Time
}

func newBroker() *broker {
	return &broker{
		subs:  make(map[string]map[*Subscription]struct{}),
		locks: make(map[string]*webhookLock),
	}
}

// withWebhookLock runs fn while holding webhookID's lock, passing it a
// timestamp at microsecond (DB) precision that is strictly later than any
// earlier one handed out under the same lock. Locks are reference-counted and
// dropped once unused, so they never outlive the requests in flight.
func (b *broker) withWebhookLock(webhookID string, fn func(stamp time.Time)) {
	b.locksMu.Lock()
	l, ok := b.locks[webhookID]
	if !ok {
		l = &webhookLock{}
		b.locks[webhookID] = l
	}
	l.refs++
	b.locksMu.Unlock()

	l.Lock()
	stamp := time.Now().UTC().Truncate(time.Microsecond)
	if !stamp.After(l.lastStamp) {
		stamp = l.lastStamp.Add(time.Microsecond)
	}
	l.lastStamp = stamp
	defer func() {
		l.Unlock()
		b.locksMu.Lock()
		l.refs--
		if l.refs == 0 {
			delete(b.locks, webhookID)
		}
		b.locksMu.Unlock()
	}()
	fn(stamp)
}

func (b *broker) subscribe(webhookID string) *Subscription {
	events := make(chan RequestEvent, subscriptionBuffer)
	s := &Subscription{Events: events, events: events, webhookID: webhookID, broker: b}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[webhookID] == nil {
		b.subs[webhookID] = make(map[*Subscription]struct{})
	}
	b.subs[webhookID][s] = struct{}{}
	return s
}

// publish delivers evt without blocking, evicting any subscriber whose
// buffer is full.
func (b *broker) publish(webhookID string, evt RequestEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs[webhookID] {
		select {
		case s.events <- evt:
		default:
			b.removeLocked(webhookID, s)
		}
	}
}

// closeWebhook ends every subscription to webhookID.
func (b *broker) closeWebhook(webhookID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for s := range b.subs[webhookID] {
		b.removeLocked(webhookID, s)
	}
}

func (b *broker) remove(webhookID string, s *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.removeLocked(webhookID, s)
}

func (b *broker) removeLocked(webhookID string, s *Subscription) {
	subs, ok := b.subs[webhookID]
	if !ok {
		return
	}
	if _, ok := subs[s]; !ok {
		return
	}
	delete(subs, s)
	close(s.events)
	if len(subs) == 0 {
		delete(b.subs, webhookID)
	}
}
