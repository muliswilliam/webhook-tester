-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
ALTER TABLE webhook_requests DROP COLUMN created_at;
ALTER TABLE webhook_requests
    ADD received_at DATETIME;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE webhook_requests
    ADD created_at DATETIME NOT NULL;
ALTER TABLE webhook_requests DROP COLUMN received_at;
-- +goose StatementEnd
