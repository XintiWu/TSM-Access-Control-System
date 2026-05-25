# 門禁系統完整 Demo 指南

專案：Distributed Physical Access Control System  
持久化：**ClickHouse only**（事件、員工、組織樹）

---

## 1. 架構一覽

```
刷卡 / curl / badge-reader-sim
        ↓
   access-api :8080  ←→ Redis（即時決策、反潛回、封禁快取）
        ↓ Kafka (inout-events)
   aggregation-worker
        ↓
   ClickHouse
   ├── inout_events          # 刷卡事件
   ├── employee              # 員工（ReplacingMergeTree，ban/unban）
   ├── org_unit              # 組織樹
   └── pre_aggregated_reports  # MV 自動預聚合

admin-api :8081
   → ClickHouse（寫入 employee 新版本）
   → Kafka (permission-events)
   → cache-invalidation-worker → Redis perm:denied

report-api :8082
   → ClickHouse（報表查詢）
   → Redis（報表快取 5 分鐘）
   → CSV / PDF 匯出
```

| 服務 | 埠 | URL |
|------|-----|-----|
| access-api | 8080 | http://localhost:8080 |
| admin-api | 8081 | http://localhost:8081 |
| report-api | 8082 | http://localhost:8082 |
| ClickHouse HTTP | 8123 | http://localhost:8123 |
| ClickHouse Native | 9000 | localhost:9000 |
| Redis | 6379 | localhost:6379 |
| Kafka | 9092 | localhost:9092 |
| Grafana | 3001 | http://localhost:3001（admin / admin） |
| Prometheus | 9090 | http://localhost:9090 |

---

## 2. 前置條件

- Docker Desktop 已啟動（`docker ps` 正常）
- 終端機在**專案根目錄**
- 建議安裝：`jq`（demo 腳本輸出 JSON）

```bash
cd "/Users/xinti/114-2/雲原生/Distributed Physical Access Control System"
```

---

## 3. 啟動與初始化

### 3.1 全新環境（第一次或 schema 變更）

```bash
make down -v    # 刪除所有 volume（含舊 ClickHouse 資料）
make up         # build + 等服務 + Kafka topics + schema-ch + seed-ch + Redis seed
```

`make up` 約 1–2 分鐘，會自動執行：

1. `docker compose up -d --build`
2. `init-kafka-topics`
3. `schema-ch` → `clickhouse/init.sql`
4. `seed-ch` → `clickhouse/seed.sql`（demo 員工與組織）
5. `seed` → Redis 卡號對應

### 3.2 日常重啟（保留資料）

```bash
docker compose up -d --build
make schema-ch seed-ch seed   # 若表或 demo 資料缺失時
```

### 3.3 健康檢查

```bash
curl -s http://localhost:8080/health | jq .
curl -s http://localhost:8081/health | jq .
curl -s http://localhost:8082/health | jq .
```

預期：`{"status":"ok"}`

### 3.4 僅重載 ClickHouse demo 資料

```bash
make seed-ch
```

---

## 4. Demo 用固定 UUID

| 用途 | UUID |
|------|------|
| 一般員工 Demo User | `22222222-2222-2222-2222-222222222222` |
| 預設封禁種子員工 | `00000000-0000-0000-0000-000000000099` |
| 門 | `11111111-1111-1111-1111-111111111111` |
| 部門 Team-A | `a0000000-0000-0000-0000-000000000003` |
| 卡號 | `CARD001` |

**Report API** 所有請求需帶：

```http
X-User-ID: 22222222-2222-2222-2222-222222222222
```

---

## 5. 推薦 Demo 流程（簡報 10–15 分鐘）

### Step 0：啟動

```bash
make up
```

**講點：** 雲原生微服務；Fast Path 與 Slow Path 分離；單一分析型資料庫 ClickHouse。

---

### Step 1：Fast Path + 非同步寫入

```bash
make verify-pipeline
```

**流程：**

1. 清除 Redis passback 狀態  
2. `POST /access/swipe`（IN）  
3. 輪詢 ClickHouse `inout_events` 直到出現 `eventId`（最長 30s）

**講點：** 刷卡決策不等待 DB；Kafka 緩衝；Worker 非同步寫入 CH。

**手動刷卡：**

```bash
make swipe
make swipe DIRECTION=OUT
```

---

### Step 2：反潛回（Anti-passback）

```bash
make demo
```

**講點：** 同一人未先 OUT 又 IN → Redis 狀態拒絕（DENY）。

---

### Step 3：管理封禁

```bash
make demo-ban
```

**流程：**

1. `POST /admin/employees/{id}/ban` → CH `employee` 插入新版本（`is_active=0`）  
2. Kafka → cache-invalidation-worker → Redis `perm:denied`  
3. 刷卡 → `DENY` / `PERMISSION_DENIED`  
4. `unban` → 再刷卡 → `ALLOW`

**單步操作：**

```bash
make ban USER=22222222-2222-2222-2222-222222222222
make swipe
make unban USER=22222222-2222-2222-2222-222222222222
make swipe
```

---

### Step 4：報表與匯出（Slow Path）

**圖表化介面（折線圖 / 長條圖 / 門禁熱點）— 建議從這裡看：**

```text
http://localhost:8082/ui/
```

瀏覽器開啟後可切換 **CEO / VP / 課長 / 員工** 身份，分頁對應：

| 分頁 | API | 圖表類型 |
|------|-----|----------|
| 部門報表 | `GET /reports/department` | 進出人次長條圖、工時/遲到率折線圖 |
| 門禁熱點 | `GET /reports/analytics/door-heatmap` | 各門流量橫向長條圖（熱點排行） |
| 考勤趨勢 | `GET /reports/analytics/attendance-trends` | 平均工時 + 遲到率折線圖 |
| 個人出勤 | `GET /reports/personal` | 個人工時長條圖（員工僅此分頁） |

> **`format=pdf&type=department`** 現已內嵌圖表（部門長條/折線、門禁熱點、考勤趨勢，約 4+ 頁）。`type=events` 的 PDF 仍僅表格。互動圖表亦可開 `/ui/`。

```bash
make demo-full          # 灌測 + 報表 API + CSV/PDF
# 或
make demo-report
```

涵蓋 API：

- 個人出勤 `/reports/personal`  
- 部門摘要 `/reports/department`  
- 稽核 `/reports/audit`  
- 同步 CSV 匯出  
- 同步 PDF  
- 非同步 PDF job  

**快速下載部門 PDF：**

```bash
make report-export-pdf
# 產生 report_export.pdf
```

**手動查部門報表：**

```bash
curl -s -H "X-User-ID: 22222222-2222-2222-2222-222222222222" \
  "http://localhost:8082/reports/department?orgUnitId=a0000000-0000-0000-0000-000000000003&startDate=$(date +%Y-%m-01)&endDate=$(date +%Y-%m-%d)&granularity=daily" | jq .
```

**講點：** `pre_aggregated_reports` 預聚合進出人次；`materialized_path` + `report_role` 分級；稽核含 `sourceIp`；支援 daily/weekly/monthly 與 PDF/CSV。

**角色 UUID（`make seed-ch` 後）：**

| 角色 | X-User-ID |
|------|-----------|
| CEO | `aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa` |
| 課長 | `cccccccc-cccc-cccc-cccc-cccccccccccc` |
| 員工 | `22222222-2222-2222-2222-222222222222` |

---

### Step 5：Grafana 分析與告警

```bash
make load-demo                              # 可選：灌測 QPS
make demo-passback-alert                    # 55 次反潛回 DENY → 觸發告警指標
```

開啟 http://localhost:3001（`admin` / `admin`）：

| 儀表板 | 內容 |
|--------|------|
| **Access Analytics** | 各門近 1h 熱點、本月平均工時/遲到率折線、反潛回 stat |
| **Shift Change Monitor** | QPS、p99、ALLOW/DENY |

告警：`report_passback_deny_1m_max >= 50`（Prometheus + Grafana Unified Alerting → `security-slack`）。請在 Grafana UI 將 Contact point 的 Slack Webhook URL 換成實際通道。

**講點：** ClickHouse datasource 驅動熱點與考勤圖；report-api 每 15s 輪詢 CH 暴露 Prometheus 指標供告警。

---

### Step 6：自動化驗證（選做）

```bash
E2E_PIPELINE=1 make test-e2e-pipeline   # 需 make up 已執行
make test-unit
```

---

### Step 7：收尾

```bash
make logs          # 追蹤 access / admin / worker / report
make down          # 停止容器
# make down -v     # 停止並刪除 volume（完全重置）
```

---

## 6. 一鍵完整腳本

```bash
make down -v && make up
make verify-pipeline
make demo
make demo-ban
make demo-report
make demo-passback-alert          # 可選：反潛回告警示範
make load-demo                    # 可選
E2E_PIPELINE=1 make test-e2e-pipeline   # 可選
```

---

## 7. 手動 API 範例

### 刷卡

```bash
curl -X POST http://localhost:8080/access/swipe \
  -H "Content-Type: application/json" \
  -d '{
    "userId": "22222222-2222-2222-2222-222222222222",
    "doorId": "11111111-1111-1111-1111-111111111111",
    "direction": "IN",
    "cardUid": "CARD001",
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }' | jq .
```

### Ban / Unban

```bash
curl -X POST http://localhost:8081/admin/employees/22222222-2222-2222-2222-222222222222/ban | jq .
curl -X POST http://localhost:8081/admin/employees/22222222-2222-2222-2222-222222222222/unban | jq .
```

### 匯出 CSV

```bash
curl -sf -H "X-User-ID: 22222222-2222-2222-2222-222222222222" \
  "http://localhost:8082/reports/export?orgUnitId=a0000000-0000-0000-0000-000000000003&startDate=$(date +%Y-%m-01)&endDate=$(date +%Y-%m-%d)&format=csv&type=department" \
  -o report.csv
```

---

## 8. ClickHouse 除錯查詢

```bash
# 最近 5 筆事件
docker compose exec clickhouse clickhouse-client --password password123 \
  --query "SELECT toString(id), toString(employee_id), direction, status
           FROM access_control.inout_events ORDER BY event_time DESC LIMIT 5"

# 員工最新狀態
docker compose exec clickhouse clickhouse-client --password password123 \
  --query "SELECT toString(id), name, argMax(is_active, updated_at) AS active
           FROM access_control.employee GROUP BY id, name"

# 組織樹
docker compose exec clickhouse clickhouse-client --password password123 \
  --query "SELECT toString(id), name, depth FROM access_control.org_unit"
```

---

## 9. 常見問題

| 現象 | 原因 | 處理 |
|------|------|------|
| `curl: (7) Failed to connect :8082` | report-api 未啟動 | `docker compose ps`；`docker compose up -d report-api` |
| `org unit not found` | CH 無 seed | `make seed-ch` |
| 報表為空 | 無刷卡事件 | 先 `make verify-pipeline` 或 `make demo-report` |
| verify-pipeline 逾時 | worker 或 Kafka 異常 | `make logs` 看 aggregation-worker |
| Docker daemon 錯誤 | Docker 未開 | 啟動 Docker Desktop |
| 舊 mariadb 容器殘留 | compose 已移除該服務 | `docker compose up -d --remove-orphans` |

---

## 10. Makefile 指令速查

| 指令 | 說明 |
|------|------|
| `make up` | 啟動全堆疊 + schema + seed |
| `make down` | 停止 |
| `make down -v` | 停止並刪 volume |
| `make schema-ch` | 套用 `clickhouse/init.sql` |
| `make seed-ch` | 載入 demo 員工/組織 |
| `make seed` | Redis 卡號 seed |
| `make verify-pipeline` | 刷卡 → CH 驗證 |
| `make demo` | 反潛回 demo |
| `make demo-ban` | 封禁流程 demo |
| `make demo-report` | 報表 API demo |
| `make swipe` | 單次刷卡模擬 |
| `make ban` / `make unban` | 封禁/解封 |
| `make load-demo` | 流量灌測（Grafana） |
| `make test-unit` | Go 單元測試 |
| `make test-e2e-pipeline` | 需 `E2E_PIPELINE=1` + `make up` |

---

## 11. 相關文件

- [README.md](README.md) — 專案概覽（已 push）
- [HANDOVER.md](HANDOVER.md) — 交接與 E2E 說明
- [manual.md](manual.md) — 詳細設計手冊
- [clickhouse/init.sql](clickhouse/init.sql) — 表結構
- [clickhouse/seed.sql](clickhouse/seed.sql) — Demo 資料
