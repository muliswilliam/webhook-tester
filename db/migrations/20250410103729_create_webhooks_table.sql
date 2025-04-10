-- +goose Up
-- +goose StatementBegin
CREATE TABLE webhooks
(
    id             TEXT PRIMARY KEY,
    title          TEXT     NOT NULL,
    response_code  INTEGER  NOT NULL DEFAULT 200,
    content_type   TEXT     NOT NULL,
    response_delay INTEGER  NOT NULL DEFAULT 0,
    payload        TEXT     NOT NULL,
    created_at     DATETIME NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS webhooks;
-- +goose StatementEnd
