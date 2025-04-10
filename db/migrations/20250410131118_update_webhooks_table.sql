-- +goose Up
-- +goose StatementBegin
ALTER TABLE webhooks ADD updated_at DATETIME;
ALTER TABLE webhooks ADD notify_on_event INTEGER DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE webhooks DROP COLUMN  updated_at;
ALTER TABLE webhooks DROP COLUMN notify_on_event;
-- +goose StatementEnd
