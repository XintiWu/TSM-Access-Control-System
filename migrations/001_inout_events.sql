CREATE TABLE IF NOT EXISTS inout_events (
  id           CHAR(36) PRIMARY KEY,
  employee_id  CHAR(36) NOT NULL,
  door_id      CHAR(36) NOT NULL,
  direction    ENUM('IN','OUT') NOT NULL,
  event_time   DATETIME(3) NOT NULL,
  status       ENUM('ALLOW','DENY') NOT NULL,
  reason       VARCHAR(32) NULL,
  source_ip    VARCHAR(45) NULL,
  card_uid     VARCHAR(64) NULL,
  created_at   DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  INDEX idx_employee_time (employee_id, event_time)
);
