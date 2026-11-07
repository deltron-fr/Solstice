-- +goose Up
ALTER TABLE listings ADD COLUMN category TEXT NOT NULL DEFAULT 'General'
    CHECK (length(trim(category)) BETWEEN 1 AND 80);
ALTER TABLE listings ADD COLUMN unit TEXT NOT NULL DEFAULT 'item'
    CHECK (length(trim(unit)) BETWEEN 1 AND 60);
ALTER TABLE listings ADD COLUMN location TEXT NOT NULL DEFAULT ''
    CHECK (length(location) <= 160);
ALTER TABLE listings ADD COLUMN image_url TEXT NOT NULL DEFAULT ''
    CHECK (image_url = '' OR image_url LIKE 'https://%');

-- +goose Down
ALTER TABLE listings DROP COLUMN image_url;
ALTER TABLE listings DROP COLUMN location;
ALTER TABLE listings DROP COLUMN unit;
ALTER TABLE listings DROP COLUMN category;
