CREATE TABLE IF NOT EXISTS employee (
  id           CHAR(36) PRIMARY KEY,
  name         VARCHAR(128) NOT NULL,
  card_uid     VARCHAR(64) NULL,
  is_active    BOOLEAN NOT NULL DEFAULT TRUE,
  org_unit_id  CHAR(36) NULL,
  updated_at   DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_employee_card_uid (card_uid)
);

INSERT INTO employee (id, name, card_uid, is_active) VALUES
  ('22222222-2222-2222-2222-222222222222', 'Demo User', 'CARD001', TRUE),
  ('00000000-0000-0000-0000-000000000099', 'Banned User', 'CARD099', FALSE)
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  card_uid = VALUES(card_uid),
  is_active = VALUES(is_active);
