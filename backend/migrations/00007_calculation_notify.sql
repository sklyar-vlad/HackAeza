-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION notify_calculation_published() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('calculation_published', coalesce(NEW.active_calculation_version, 0)::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER published_state_notify AFTER UPDATE ON published_state
    FOR EACH ROW WHEN (OLD.active_calculation_version IS DISTINCT FROM NEW.active_calculation_version)
    EXECUTE FUNCTION notify_calculation_published();

-- +goose Down
DROP TRIGGER published_state_notify ON published_state;
DROP FUNCTION notify_calculation_published();
