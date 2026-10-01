# Webhook Tester

Webhook Tester gives developers unique endpoints that capture the requests webhook providers send, answer them with a configured response, and relay them to the developer's own server.

## Language

### Endpoints and capture

**Webhook**:
An endpoint with a unique URL, its configured response and, optionally, a forward URL.
_Avoid_: Hook, bin, channel

**Guest webhook**:
A webhook without an owner, created for a visitor who isn't signed in. It expires after a while and never forwards, so it can't be used as an open relay.
_Avoid_: Public webhook, anonymous webhook

**Owner**:
The signed-in user a webhook belongs to. Only webhooks with an owner can forward.

**Captured request**:
One request a webhook received, stored with its method, subpath, query, headers and body exactly as sent (the `WebhookRequest` model).
_Avoid_: Event, hit, webhook request

**Subpath**:
The part of a captured request's path after the webhook's URL, such as `/orders/42`.

**Configured response**:
The status, content type, headers, payload and delay a webhook answers every captured request with. Forwarding never changes it.

### Forwarding

**Forward URL**:
The absolute http or https URL a webhook relays its captured requests to, with the subpath appended and the query merged. It can't point back at this instance's own webhooks, nor, by default, at a private or local address.
_Avoid_: Target, destination, callback URL

**Forward**:
Relaying a captured request to the forward URL: same method, subpath, query, headers and body, minus hop-by-hop headers, plus the `X-Webhook-Tester-Request-Id` header. A captured request that carries that header came back from a forward and isn't forwarded again. Nor is one whose subpath any server could read as holding a `..` segment, encoded or not, since it could reach paths outside the forward URL's; its delivery records why.
_Avoid_: Proxy, relay request

**Delivery**:
One attempt to forward a captured request, recording the target URL, the answer's status, headers and truncated body or the error, and how long it took. A captured request keeps its latest 50.
_Avoid_: Attempt, forward result

**Trigger**:
What started a delivery: `auto`, when the request was captured, or `replay`, when a user replayed it to the forward URL.

**Delivery outcome**:
How a delivery ended: `2xx`, `3xx`, `4xx` or `5xx` when the target answered, `error` when it couldn't be reached, `blocked` when the destination was refused, or `dropped` when forwarding was saturated or shutting down.
_Avoid_: Status, result

### Replay

**Replay**:
Sending a captured request again, to one of two replay targets.

**Replay target**:
Where a replay goes: the `endpoint`, which captures a copy as a new captured request, or the `forward` URL, which records a replay delivery on the original instead.
