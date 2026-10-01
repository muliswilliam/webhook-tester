# Guard forwards against private destinations at dial time, and check them again when saved

Forwarding makes the server send requests to URLs users choose, so on a public instance it could be used to reach internal services or cloud metadata endpoints (SSRF). The forwarder's dialer checks every resolved address just before connecting and refuses private, loopback, link-local, reserved and non-global IPv6 addresses, which also catches hostnames that resolve or are rebound to them; redirects aren't followed, so they can't lead past it. A refused forward is recorded as a `blocked` delivery. Checking only the URL when it's saved wouldn't be enough, since DNS can change after the check.

On top of that, a forward URL being set or changed is rejected when its host is, or only resolves to, such an address, so the user learns about it while editing instead of from blocked deliveries. That check is advisory: a failed or slow lookup passes, and an unchanged forward URL isn't checked again, so a host whose DNS changed later doesn't stop the owner editing the webhook's other settings.

`FORWARD_ALLOW_PRIVATE_NETWORKS` turns both checks off, for self-hosted instances that forward to `localhost` or their own network. It is off by default and must stay off on a public instance. Guest webhooks never forward at all, so only signed-in users can make the server send requests.
