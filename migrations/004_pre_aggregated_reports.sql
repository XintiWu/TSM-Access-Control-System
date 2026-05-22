CREATE TABLE IF NOT EXISTS pre_aggregated_reports (
  id               CHAR(36) PRIMARY KEY,
  org_unit_id      CHAR(36) NOT NULL,
  report_date      DATE NOT NULL,
  total_entries    INT NOT NULL DEFAULT 0,
  total_exits      INT NOT NULL DEFAULT 0,
  unique_employees INT NOT NULL DEFAULT 0,
  avg_hours        DECIMAL(5,2) NULL,
  computed_at      DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_org_date (org_unit_id, report_date),
  INDEX idx_org_unit (org_unit_id),
  INDEX idx_date (report_date)
);
