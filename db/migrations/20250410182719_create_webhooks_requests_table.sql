-- +goose Up
-- +goose StatementBegin
CREATE TABLE webhook_requests
(
    id     TEXT PRIMARY KEY,
    webhook_id TEXT NOT NULL,
    method TEXT NOT NULL,
    body   TEXT,
    headers TEXT,
    query TEXT,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (webhook_id) REFERENCES webhooks(id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS webhook_requests;
-- +goose StatementEnd
