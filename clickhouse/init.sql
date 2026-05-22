CREATE DATABASE IF NOT EXISTS access_control;

-- Raw events table in ClickHouse
CREATE TABLE IF NOT EXISTS access_control.inout_events (
    id UUID,
    employee_id UUID,
    door_id UUID,
    direction Enum8('IN' = 1, 'OUT' = 2),
    event_time DateTime64(3, 'UTC'),
    status Enum8('ALLOW' = 1, 'DENY' = 2),
    reason Nullable(String),
    source_ip String,
    card_uid String,
    org_unit_id UUID
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_time)
ORDER BY (org_unit_id, event_time, employee_id)
SETTINGS index_granularity = 8192;

-- Master data: doors
CREATE TABLE IF NOT EXISTS access_control.door (
    id UUID,
    name String,
    site String DEFAULT 'Hsinchu'
) ENGINE = MergeTree()
ORDER BY id;

-- Pre-aggregated reports (entries/exits/headcount) via AggregatingMergeTree
CREATE TABLE IF NOT EXISTS access_control.pre_aggregated_reports (
    org_unit_id UUID,
    report_date Date,
    total_entries SimpleAggregateFunction(sum, UInt64),
    total_exits SimpleAggregateFunction(sum, UInt64),
    unique_employees AggregateFunction(uniq, UUID)
) ENGINE = AggregatingMergeTree()
PRIMARY KEY (org_unit_id, report_date)
ORDER BY (org_unit_id, report_date);

CREATE MATERIALIZED VIEW IF NOT EXISTS access_control.mv_pre_aggregated_reports
TO access_control.pre_aggregated_reports AS
SELECT
    org_unit_id,
    toDate(event_time) AS report_date,
    countIf(direction = 'IN') AS total_entries,
    countIf(direction = 'OUT') AS total_exits,
    uniqState(employee_id) AS unique_employees
FROM access_control.inout_events
WHERE status = 'ALLOW'
GROUP BY org_unit_id, report_date;

-- Per-door traffic (all swipes) for heatmap / real-time dashboards
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

-- Master data: org tree (read-mostly)
CREATE TABLE IF NOT EXISTS access_control.org_unit (
    id UUID,
    name String,
    parent_id Nullable(UUID),
    depth UInt8,
    materialized_path String
) ENGINE = MergeTree()
ORDER BY materialized_path;

-- Master data: employees (ban/unban via ReplacingMergeTree)
CREATE TABLE IF NOT EXISTS access_control.employee (
    id UUID,
    name String,
    card_uid Nullable(String),
    is_active UInt8,
    org_unit_id Nullable(UUID),
    report_role LowCardinality(String) DEFAULT 'EMPLOYEE',
    updated_at DateTime64(3, 'UTC')
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY id;
