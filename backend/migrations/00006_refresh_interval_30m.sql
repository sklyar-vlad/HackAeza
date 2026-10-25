-- +goose Up
UPDATE model_versions SET refresh_interval_seconds = 1800 WHERE refresh_interval_seconds = 60;

-- +goose Down
UPDATE model_versions SET refresh_interval_seconds = 60 WHERE refresh_interval_seconds = 1800;
