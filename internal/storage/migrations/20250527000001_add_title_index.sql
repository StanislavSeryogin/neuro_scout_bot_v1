-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_articles_title_trgm ON articles USING gin (title gin_trgm_ops);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_articles_title_trgm;
-- +goose StatementEnd 