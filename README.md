# 🧪 Webhook Tester

A lightweight, developer-friendly platform for testing and debugging webhooks - built in Go.

This project allows developers to create unique webhook endpoints, capture incoming requests, inspect headers and
payloads, and optionally replay those requests.

---

## ✨ Features

- 📩 Receive webhooks at unique URLs
- 🔍 Inspect request payloads (headers, body, method, query params)
- 💾 Log and view webhook events in real-time
- 🛠️ Customize responses (status code, content type, payload, delay)
- 🔁 Replay events
- ↪️ Forward captured requests to your own server, and replay them to it after a fix. Each attempt is recorded as a delivery, and a request keeps its latest 50
- 🔐 API to manage webhooks
- 📚 Swagger API documentation
- 🧪 Built for testing, mocking, and debugging external integrations

---

## 🏃‍♂️ Getting Started (Manual)

### 1. Clone & Configure

```bash
git clone https://github.com/muliswilliam/webhook-tester
cd webhook-tester
cp .env.example .env
```

### 2. Generate AUTH_SECRET

Auth secret must be 32 bytes. Generate one with:

```bash
openssl rand -base64 32
```
Paste it into your .env file.

### 3. Optional settings

- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`: SMTP server for password reset emails. Without `SMTP_HOST`, emails are written to the server log instead.
- `METRICS_ADDR` (default `:9091`): internal address serving Prometheus metrics at `/metrics`. Don't expose it publicly.
- `FORWARD_ALLOW_PRIVATE_NETWORKS` (default `false`): let forwarding reach private, loopback and link-local addresses, such as `localhost` or services on your own network. Keep it off on a public instance, or anyone with an account can make the server call into your internal network. While it's off, a forward URL that points at such an address is rejected when it's saved.
- `FORWARD_TIMEOUT` (default `10s`): how long a forward to a webhook's forward URL may take, as a Go duration (`500ms`, `30s`).
- `FORWARD_MAX_CONCURRENT` (default `32`): how many automatic forwards may be in flight at once. Captured requests beyond that still get captured, and their delivery records a "forwarding queue full" error.

### 4. Run Locally

```bash
go run cmd/main.go
```

Visit: http://localhost:3000

---

## 🐳 Running with Docker (Recommended)

1. Set environment variables & Start Services
```bash
cp .env.example .env
make up
```

This builds the Docker image and starts all services using docker-compose.

2. View Logs for a Specific Service
```bash
make logs SERVICE=app
make logs SERVICE=db
```

3. Restart a Specific Service

```bash
make restart SERVICE=app
make restart SERVICE=db
```

4. Stop All Services
```bash
make down
```

5. List Available Services
```bash
make services
```

6. View Available Commands

Run the help command to see what you can do with `make`:

```bash
make help
```

---

📁 Project Structure

cmd/              # App entrypoint
internal/         # Handlers, models, db logic, templates
docs/             # Swagger documentation
static/           # JS, icons, etc.
db/migrations/    # SQL migrations
Makefile          # Dev & deployment automation
Dockerfile        # Production build config
docker-compose.yml

---

📚 API Documentation

After running the app, access Swagger docs at:

http://localhost:3000/docs

API endpoints require a valid API key sent via X-API-Key header.

---

📌 Roadmap
- ✅ API Authentication with API keys
- ✅ Swagger Documentation
- ✅ Docker + Compose for deployment
- ⏳ Email notifications on request
- ⏳ Rate limiting and abuse protection
- ⏳ Metrics and observability (LGTM stack)
- ⏳ Export logs to JSON/CSV
- ⏳ Team/organization mode for sharing webhooks

---

🧠 Credits

Developed with ❤️ by William Muli