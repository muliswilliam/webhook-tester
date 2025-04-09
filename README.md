🧪 Webhook Tester

A lightweight, developer-friendly platform for testing and debugging webhooks — built in Go.

This project allows developers to create unique webhook endpoints, capture incoming requests, inspect headers and payloads, and optionally replay those requests to other destinations.

⸻

✨ Features
	•	📩 Receive Webhooks at unique, session-based URLs
	•	🔍 Inspect Request Payloads (headers, body, method, query params)
	•	💾 In-Memory Logging of webhook events
	•	🛠️ Custom Responses (set status code, headers, body, and delay)
	•	🔁 Replay Events to any external URL
	•	🧱 RESTful API-first Architecture (UI coming soon)
	•	🧪 Designed for testing, mocking, and debugging external integrations

⸻

📦 API Structure

Method	Endpoint	Description
POST	/api/webhooks	Create a new webhook session
GET	/api/webhooks	List all sessions
GET	/api/webhooks/{id}	View a specific webhook session
PUT	/api/webhooks/{id}	Update response config
DELETE	/api/webhooks/{id}	Delete a webhook session
POST	/api/webhooks/{id}/events	Receive a webhook event
GET	/api/webhooks/{id}/events	View captured events
POST	/api/webhooks/{id}/replay	Replay captured events



⸻

🏃‍♂️ Getting Started

git clone https://github.com/muliswilliam/webhook-tester
cd webhook-tester
go run cmd/server/main.go

The server runs on http://localhost:3000

⸻

📌 Roadmap
	•	SQLite/PostgreSQL support for persistence
	•	Authentication and API keys
	•	Replay history with status tracking
	•	Web UI for viewing and managing sessions
	•	Deployable Docker image

⸻
