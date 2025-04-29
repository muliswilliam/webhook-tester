🧪 Webhook Tester

A lightweight, developer-friendly platform for testing and debugging webhooks — built in Go.

This project allows developers to create unique webhook endpoints, capture incoming requests, inspect headers and
payloads, and optionally replay those requests to other destinations.

⸻

✨ Features

- 📩 Receive Webhooks at unique, session-based URLs
- 🔍 Inspect Request Payloads (headers, body, method, query params)
- 💾 Logging of webhook events
- 🛠️ Custom Responses (set status code, headers, body, and delay)
- 🔁 Replay Events to any external URL
- 🧱 RESTful API-first Architecture (Docs coming soon)
- 🧪 Designed for testing, mocking, and debugging external integrations

⸻

🏃‍♂️ Getting Started

```
git clone https://github.com/muliswilliam/webhook-tester
cd webhook-tester
cp .env.example .env
go run cmd/main.go
```

The server runs on http://localhost:3000

Generating AUTH_SECRET
Auth secret must be 32 bytes, generate one using openssl:

```
openssl rand -base64 32
```

⸻

📌 Roadmap

- API Authentication using API keys
- API Documentation
- Deployable Docker image
