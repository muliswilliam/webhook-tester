package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"webhook-tester/config"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/models"
	"webhook-tester/internal/utils"
)

// RequestIDHeader is added to every forwarded request, carrying the ID of
// the captured request, so the target's logs can be matched to it. A
// captured request that has it is a forward that came back, and isn't
// forwarded again.
const RequestIDHeader = "X-Webhook-Tester-Request-Id"

// Delivery errors recorded for forwards that weren't attempted.
const (
	// ErrMsgQueueFull is the error of an automatic forward dropped because
	// the maximum number of forwards were already in flight.
	ErrMsgQueueFull = "forwarding queue full"
	// ErrMsgShuttingDown is the error of an automatic forward dropped
	// because the server was shutting down.
	ErrMsgShuttingDown = "server shutting down"
	// ErrMsgDestinationNotAllowed starts the error of a forward whose
	// destination resolved to an address forwarding may not reach.
	ErrMsgDestinationNotAllowed = "destination not allowed"
)

// errDestinationNotAllowed is returned by the dial guard for an address
// forwarding may not reach.
var errDestinationNotAllowed = errors.New(ErrMsgDestinationNotAllowed)

// unforwardedHeaders are the captured headers never relayed: the hop-by-hop
// ones, which apply to a single connection (RFC 9110 section 7.6.1), and
// Host and Content-Length, which the client sets for the outgoing request.
var unforwardedHeaders = map[string]bool{
	"Connection":          true,
	"Proxy-Connection":    true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
	"Host":                true,
	"Content-Length":      true,
}

// DeliveryRecorder stores deliveries and publishes them to live
// subscribers. WebhookService implements it.
type DeliveryRecorder interface {
	RecordDelivery(d *models.Delivery) error
}

// Forwarder relays captured requests to their webhook's forward URL and
// records each attempt as a delivery.
type Forwarder struct {
	client   *http.Client
	timeout  time.Duration
	recorder DeliveryRecorder
	metrics  metrics.Recorder
	logger   *log.Logger

	// slots bounds the automatic forwards in flight.
	slots chan struct{}
	// mu guards closed, and orders inFlight.Add before Shutdown's Wait:
	// once closed is set, no automatic forward is started.
	mu       sync.Mutex
	closed   bool
	inFlight sync.WaitGroup
}

// NewForwarder builds a Forwarder from the forwarding settings. A zero
// Timeout or MaxConcurrent falls back to the config defaults.
func NewForwarder(
	cfg config.Forwarding,
	recorder DeliveryRecorder,
	rec metrics.Recorder,
	logger *log.Logger,
) *Forwarder {
	if cfg.Timeout <= 0 {
		cfg.Timeout = config.DefaultForwardTimeout
	}
	if cfg.MaxConcurrent < 1 {
		cfg.MaxConcurrent = config.DefaultForwardMaxConcurrent
	}
	return &Forwarder{
		client:   newForwardClient(cfg),
		timeout:  cfg.Timeout,
		recorder: recorder,
		metrics:  rec,
		logger:   logger,
		slots:    make(chan struct{}, cfg.MaxConcurrent),
	}
}

// newForwardClient returns the client forwards are sent with: bounded by the
// timeout and the response header cap, without compression (which would add an Accept-Encoding header
// the original request didn't have), without proxies, and without following
// redirects, so a redirect is recorded as the answer and can't lead the
// request past the dial guard's checks to a different host.
func newForwardClient(cfg config.Forwarding) *http.Client {
	dialer := &net.Dialer{Timeout: cfg.Timeout, KeepAlive: 30 * time.Second}
	if !cfg.AllowPrivateNetworks {
		dialer.Control = denyPrivateAddresses
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = dialer.DialContext
	transport.DisableCompression = true
	transport.MaxResponseHeaderBytes = models.MaxDeliveryResponseHeaders
	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// denyPrivateAddresses is a net.Dialer Control function. It runs for each
// resolved address just before connecting, so it also catches hostnames that
// resolve, or are rebound, to an address forwarding mustn't reach.
func denyPrivateAddresses(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errDestinationNotAllowed
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !isPublicAddr(ip) {
		return errDestinationNotAllowed
	}
	return nil
}

// Forward relays wr to wh's forward URL, records the attempt as a delivery
// with the given trigger, publishes it and returns it. Every outcome,
// including a network error, is a delivery; failing to store it is only
// logged, since that happens when wr was deleted while it was in flight.
func (f *Forwarder) Forward(ctx context.Context, wh models.Webhook, wr models.WebhookRequest, trigger models.DeliveryTrigger) models.Delivery {
	d := newDelivery(wr, trigger)
	d.Outcome = f.send(ctx, wh, wr, &d)
	f.record(&d)
	return d
}

// ForwardAsync forwards wr in the background with trigger auto. If the
// maximum number of forwards are already in flight, or Shutdown was called,
// it records a delivery saying why instead of forwarding.
func (f *Forwarder) ForwardAsync(wh models.Webhook, wr models.WebhookRequest) {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		f.refuse(wh, wr, ErrMsgShuttingDown)
		return
	}
	select {
	case f.slots <- struct{}{}:
	default:
		f.mu.Unlock()
		f.refuse(wh, wr, ErrMsgQueueFull)
		return
	}
	f.inFlight.Add(1)
	f.mu.Unlock()

	go func() {
		defer func() {
			<-f.slots
			f.inFlight.Done()
		}()
		// Not tied to the capture's request context, which ends as soon as
		// the provider has its response.
		f.Forward(context.Background(), wh, wr, models.DeliveryTriggerAuto)
	}()
}

// refuse records an automatic delivery of wr that wasn't attempted, with
// reason as its error.
func (f *Forwarder) refuse(wh models.Webhook, wr models.WebhookRequest, reason string) {
	d := newDelivery(wr, models.DeliveryTriggerAuto)
	if wh.ForwardURL != nil {
		d.TargetURL, _ = wr.URLAt(*wh.ForwardURL)
	}
	d.Error = ptr(reason)
	d.Outcome = models.DeliveryOutcomeDropped
	f.record(&d)
}

// Shutdown stops starting automatic forwards - later ones are recorded as
// refused - and waits until those in flight have been recorded, or ctx is
// done. Synchronous forwards (Forward) still work afterwards.
func (f *Forwarder) Shutdown(ctx context.Context) error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()

	done := make(chan struct{})
	go func() {
		f.inFlight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// newDelivery starts the delivery of wr with the given trigger.
func newDelivery(wr models.WebhookRequest, trigger models.DeliveryTrigger) models.Delivery {
	return models.Delivery{
		ID:        utils.GenerateID(),
		RequestID: wr.ID,
		WebhookID: wr.WebhookID,
		Trigger:   trigger,
		StartedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
}

// send makes the outbound request and fills in d with what came back,
// returning its outcome.
func (f *Forwarder) send(ctx context.Context, wh models.Webhook, wr models.WebhookRequest, d *models.Delivery) models.DeliveryOutcome {
	start := time.Now()
	defer func() { d.DurationMs = time.Since(start).Milliseconds() }()

	if wh.ForwardURL == nil {
		d.Error = ptr("the webhook has no forward URL")
		return models.DeliveryOutcomeError
	}
	target, err := wr.URLAt(*wh.ForwardURL)
	if err != nil {
		d.Error = ptr(fmt.Sprintf("invalid forward URL: %v", err))
		return models.DeliveryOutcomeError
	}
	d.TargetURL = target

	req, err := http.NewRequestWithContext(ctx, wr.Method, target, strings.NewReader(wr.Body))
	if err != nil {
		d.Error = ptr(fmt.Sprintf("couldn't build the request: %v", err))
		return models.DeliveryOutcomeError
	}
	req.Header = forwardHeaders(wr)

	resp, err := f.client.Do(req)
	if err != nil {
		if errors.Is(err, errDestinationNotAllowed) {
			d.Error = ptr(blockedMessage(req.URL.Hostname()))
			return models.DeliveryOutcomeBlocked
		}
		d.Error = ptr(f.describeError(err))
		return models.DeliveryOutcomeError
	}
	defer func() { _ = resp.Body.Close() }()

	d.StatusCode = &resp.StatusCode
	d.ResponseHeaders = models.CapturedValues(resp.Header)

	body, err := io.ReadAll(io.LimitReader(resp.Body, models.MaxDeliveryResponseBody+1))
	if len(body) > models.MaxDeliveryResponseBody {
		body = body[:models.MaxDeliveryResponseBody]
		d.ResponseBodyTruncated = true
	}
	d.ResponseBody = storableText(body)
	if err != nil {
		d.Error = ptr("reading the response body: " + f.describeError(err))
	}
	return models.DeliveryOutcomeForStatus(resp.StatusCode)
}

// record counts, stores and publishes the delivery. The metrics count every
// forward attempt, refused ones included, whether or not its delivery can be
// stored: they measure forwarding itself, and a delivery is only lost when
// its request was deleted meanwhile or the DB failed, which is logged.
func (f *Forwarder) record(d *models.Delivery) {
	f.metrics.ObserveDelivery(d.Outcome, time.Duration(d.DurationMs)*time.Millisecond)
	if err := f.recorder.RecordDelivery(d); err != nil {
		f.logger.Printf("forward: delivery %s of request %s (deleted meanwhile?): %v", d.ID, d.RequestID, err)
	}
}

// forwardHeaders are the captured request's headers, every value of each,
// minus the unforwarded ones and any the Connection header names, plus
// RequestIDHeader.
func forwardHeaders(wr models.WebhookRequest) http.Header {
	captured := wr.HeaderValues()
	drop := map[string]bool{}
	for _, c := range captured.Values("Connection") {
		for _, name := range strings.Split(c, ",") {
			drop[textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name))] = true
		}
	}

	h := http.Header{}
	for k, values := range captured {
		key := textproto.CanonicalMIMEHeaderKey(k)
		if unforwardedHeaders[key] || drop[key] {
			continue
		}
		h[key] = append(h[key], values...)
	}
	// Go adds its own User-Agent unless one is set; an empty value sends
	// none, as the original request did.
	if _, ok := h["User-Agent"]; !ok {
		h.Set("User-Agent", "")
	}
	h.Set(RequestIDHeader, wr.ID)
	return h
}

// blockedMessage explains why forwarding to host was refused.
func blockedMessage(host string) string {
	verb := "resolves to"
	if _, err := netip.ParseAddr(host); err == nil {
		verb = "is"
	}
	return fmt.Sprintf("%s: %s %s a private or reserved address", ErrMsgDestinationNotAllowed, host, verb)
}

// describeError turns a client error into a short message saying why the
// target wasn't reached.
func (f *Forwarder) describeError(err error) string {
	var (
		dnsErr  *net.DNSError
		certErr *tls.CertificateVerificationError
		uaErr   x509.UnknownAuthorityError
		hostErr x509.HostnameError
		invErr  x509.CertificateInvalidError
		recErr  tls.RecordHeaderError
		netErr  net.Error
	)
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled before the target answered"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return fmt.Sprintf("timed out after %s", f.timeout)
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused"
	case errors.Is(err, syscall.ECONNRESET):
		return "connection reset by the target"
	case errors.As(err, &dnsErr):
		if dnsErr.IsNotFound {
			return fmt.Sprintf("DNS lookup failed: no such host %s", dnsErr.Name)
		}
		return fmt.Sprintf("DNS lookup failed for %s: %s", dnsErr.Name, dnsErr.Err)
	case errors.As(err, &certErr), errors.As(err, &uaErr), errors.As(err, &hostErr), errors.As(err, &invErr):
		return "TLS certificate not accepted: " + innermost(err).Error()
	case errors.As(err, &recErr):
		return "TLS handshake failed: the target didn't answer with TLS"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "the target closed the connection without answering"
	case strings.Contains(err.Error(), "server response headers exceeded"):
		// net/http reports the cap with an untyped error.
		return fmt.Sprintf("the target's response headers exceeded %d KiB", models.MaxDeliveryResponseHeaders>>10)
	}
	return innermost(err).Error()
}

// innermost drops the url.Error wrapper, whose message repeats the method
// and URL.
func innermost(err error) error {
	var uErr *url.Error
	if errors.As(err, &uErr) {
		return uErr.Err
	}
	return err
}

// storableText makes a response body safe to store as text: Postgres
// rejects invalid UTF-8 and NUL bytes. A body truncated mid-character loses
// only that character.
func storableText(b []byte) string {
	s := strings.ToValidUTF8(string(b), "�")
	return strings.ReplaceAll(s, "\x00", "�")
}

func ptr[T any](v T) *T { return &v }
