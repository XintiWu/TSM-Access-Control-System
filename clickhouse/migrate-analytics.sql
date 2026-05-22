-- Idempotent upgrades for existing ClickHouse volumes (run: make schema-ch-migrate)

CREATE TABLE IF NOT EXISTS access_control.door (
    id UUID,
    name String,
    site String DEFAULT 'Hsinchu'
) ENGINE = MergeTree()
ORDER BY id;

ALTER TABLE access_control.employee
    ADD COLUMN IF NOT EXISTS report_role LowCardinality(String) DEFAULT 'EMPLOYEE';

CREATE TABLE IF NOT EXISTS access_control.door_traffic_minute (
    door_id UUID,
    minute DateTime,
    swipe_count SimpleAggregateFunction(sum, UInt64),
    deny_passback_count SimpleAggregateFunction(sum, UInt64)
) ENGINE = AggregatingMergeTree()
ORDER BY (door_id, minute);

CREATE MATERIALIZED VIEW IF NOT EXISTS access_control.mv_door_traffic_minute
TO access_control.door_traffic_minute AS
SELECT
    door_id,
    toStartOfMinute(event_time) AS minute,
    count() AS swipe_count,
    countIf(status = 'DENY' AND reason = 'ANTI_PASSBACK') AS deny_passback_count
FROM access_control.inout_events
GROUP BY door_id, minute;
