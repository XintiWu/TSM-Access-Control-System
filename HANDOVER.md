# 專案交接文件 — Distributed Physical Access Control System

**交接日期：** 2026-05-19  
**範圍：** Phase 1 Fast Path + Phase 2 Admin 封禁鏈路  
**最後更新：** 2026-05-19（Phase 2 Admin 封禁鏈路、Kafka→DB 驗證與 E2E 測試）

---

## 1. 專案目標（簡述）

為 TSMC 規模（約 9 萬員工）的實體門禁系統，實作 **刷卡當下開門決策**（Fast Path）：

- 決策延遲目標：**&lt; 50ms**（僅依賴 Redis，不查主資料庫）
- **反潛回（Anti-Passback）**：禁止連續兩次 IN 或連續兩次 OUT
- **非同步寫入**：刷卡事件經 Kafka 緩衝，再由 Worker 寫入 MariaDB
- **韌性**：主資料庫掛掉時，門仍可開（決策與事件發布不依賴 DB）

已涵蓋 **Access Tier 刷卡路徑** 與 **Admin 封禁 / Cache Invalidation**；尚未含 Report API、K8s 維運。

---

## 2. 目錄結構

```
Distributed Physical Access Control System/
├── docker-compose.yml          # 本地一鍵啟動全部服務
├── Makefile                    # up / migrate / ban / demo-ban / verify-pipeline / test
├── README.md                   
├── migrations/
│   ├── 001_inout_events.sql
│   └── 002_employee.sql        # 員工主資料 + demo seed
├── scripts/
│   ├── seed-redis.sh
│   ├── demo.sh
│   ├── demo-ban.sh             # Admin 封禁鏈路示範
│   └── verify-pipeline.sh
├── access-api/
│   └── tests/integration/      # swipe 整合測試、Kafka→DB E2E
├── admin-api/                  # 封禁/解封 REST
├── cache-invalidation-worker/  # permission-events → Redis
├── aggregation-worker/
└── badge-reader-sim/
```

**注意：** 各 Go 服務有獨立 `go.mod`；根目錄請用 `make` 目標。

---

## 3. 已完成項目

### 3.1 基礎設施

| 項目 | 說明 |
|------|------|
| Docker Compose | Redis、Kafka、MariaDB、access-api、admin-api、aggregation-worker、cache-invalidation-worker |
| MariaDB 初始化 | `001_inout_events.sql`、`002_employee.sql`（新 volume 自動執行；舊 volume 用 `make migrate`） |
| Makefile | 含 `init-kafka-topics`、`migrate`、`ban`/`unban`、`demo-ban`、`verify-pipeline`、`test-e2e-pipeline` 等 |
| 測試資料腳本 | `seed-redis.sh`（僅 door 狀態與 passback；**不**寫入 `perm:denied`） |
| Kafka Topics | `make up` 時建立 `inout-events`、`permission-events`，並重啟 cache-invalidation-worker |

**埠號（本機）：**

| 服務 | 埠 |
|------|-----|
| Access API | 8080 |
| Admin API | 8081 |
| Redis | 6379 |
| Kafka | 9092 |
| MariaDB | **3307**（對外；因本機 3306 常被佔用） |

### 3.2 Access API（`access-api/`）

| 功能 | 路徑 | 狀態 |
|------|------|------|
| 刷卡決策 | `POST /access/swipe` | 完成 |
| 員工進出狀態 | `GET /access/employee/{userId}/state` | 完成 |
| 門禁線上狀態 | `GET /access/door/{doorId}/status` | 完成 |
| 健康檢查 | `GET /health` | 完成 |
| Prometheus 指標 | `GET /metrics` | 完成（基礎 counter / histogram） |
| Grafana 儀表板 | `monitoring/grafana/dashboards/` | 完成（Shift Change：QPS、p99、ALLOW/DENY） |
| Prometheus + Grafana 服務 | `docker-compose` prometheus/grafana | 完成（:9090 / :3000） |

**決策邏輯（`internal/service/access_decision.go`）：**

1. `EXISTS perm:denied:{userId}` → 有則 `DENY` + `PERMISSION_DENIED`
2. 讀 `passback:{userId}`（無 key 視為 `NONE`）做反潛回：
   - `IN` + 目前已是 `IN` → `DENY` + `ANTI_PASSBACK`
   - `OUT` + 目前已是 `OUT` → `DENY` + `ANTI_PASSBACK`
3. `ALLOW` 時 `SET passback:{userId}` = 本次方向（TTL 24h）
4. 產生 `eventId`，**非同步**發布至 Kafka（失敗不影響 HTTP 200 與開門決策）

**Redis Key 設計（與設計文件 §4.1.2 一致）：**

| Key | 值 | 說明 |
|-----|-----|------|
| `perm:denied:{userId}` | `DENY` | 僅快取「被封禁」者；**key 不存在 = 允許** |
| `passback:{userId}` | `IN` / `OUT` | 反潛回狀態 |
| `door:status:{doorId}` | `ONLINE` / `OFFLINE` | 門禁心跳（TTL 30s） |

### 3.3 訊息佇列與 Worker

| 項目 | 說明 |
|------|------|
| Kafka Topic `inout-events` | Access API 發布刷卡事件 |
| Kafka Topic `permission-events` | Admin API 發布封禁/解封事件 |
| 刷卡事件 Schema | JSON：`eventId`, `employeeId`, `doorId`, `direction`, `eventTime`, `status`, `reason`, `cardUid`, `sourceIp` |
| 權限事件 Schema | JSON：`userId`, `action`（`BAN`/`UNBAN`）, `eventTime` |
| Producer | `access-api`、`admin-api` 各自 `internal/queue/kafka_producer.go` |
| Aggregation Worker | 消費 `inout-events` → `INSERT IGNORE INTO inout_events` |
| Cache Invalidation Worker | 消費 `permission-events` → 更新 Redis `perm:denied:{userId}` |

### 3.4 Admin 封禁鏈路（Phase 2）

| 項目 | 說明 |
|------|------|
| Admin API | `POST /admin/employees/{userId}/ban`、`/unban`（埠 8081） |
| DB | `employee` 表（`migrations/002_employee.sql`），更新 `is_active` |
| Kafka | Topic `permission-events` |
| Cache Invalidation Worker | BAN → `SET perm:denied:{userId} DENY EX 86400`；UNBAN → `DEL` |
| 示範 | `make demo-ban`、`make ban` / `make unban` |

**封禁流程：**

```
Admin API: UPDATE employee.is_active
    → Kafka permission-events
    → cache-invalidation-worker
    → Redis perm:denied:{userId}
    → 下次 access-api 刷卡 → PERMISSION_DENIED
```

**注意：** `demo.sh` 第四步會先呼叫 Admin API 封禁 `00000000-...099`，再刷卡驗證；不再依賴 `seed-redis.sh` 寫入 deny key。

### 3.5 刷卡機模擬器（`badge-reader-sim/`）

- CLI：`go run ./cmd/sim`，支援 `--direction`、`--count`、`--interval` 壓測
- 或根目錄：`make swipe` / `make swipe DIRECTION=OUT`

### 3.6 Kafka → DB 鏈路驗證與自動化測試

驗證 **Access API 發布 Kafka → Aggregation Worker 消費 → MariaDB `inout_events`** 整條非同步寫入路徑。

| 方式 | 指令 | 前置條件 | 說明 |
|------|------|----------|------|
| Shell 腳本驗證 | `make verify-pipeline` | `make up` 後五個服務皆 Running | `scripts/verify-pipeline.sh`：清 passback → swipe → 輪詢 DB 比對 eventId / employee / direction / status |
| Go 整合測試（Redis） | `make test-integration` | Docker（testcontainers 起 Redis） | **不需** compose 全堆疊；`swipe_test.go` 驗證反潛回 IN→IN→OUT |
| Go 全鏈路 E2E | `make test-e2e-pipeline` | `make up` + 設 `E2E_PIPELINE=1` | `pipeline_e2e_test.go`：HTTP swipe → 直連 MariaDB 輪詢至 30s |
| 單元測試 | `make test-unit` | 無 | `access_decision_test.go` 等，不依賴外部服務 |

**驗證流程（`verify-pipeline` / E2E 共用邏輯）：**

```
POST /access/swipe (IN)
    → access-api 回傳 eventId、decision
    → Kafka topic inout-events
    → aggregation-worker INSERT IGNORE inout_events
    → 輪詢 MariaDB WHERE id = eventId（預設每 2s、最長 30s）
    → 斷言 employee_id、direction、status 與 swipe 回應一致
```

**相關檔案：**

| 檔案 | 用途 |
|------|------|
| `scripts/verify-pipeline.sh` | 手動／CI 友善的 Bash 鏈路驗證 |
| `access-api/tests/integration/pipeline_e2e_test.go` | `TestPipelineE2E`（build tag `integration`） |
| `access-api/tests/integration/swipe_test.go` | `TestSwipeIntegration`（testcontainers Redis） |
| `access-api/internal/service/access_decision_test.go` | 決策邏輯單元測試 |
| `Makefile` | `test-unit` / `test-integration` / `test-e2e-pipeline` / `verify-pipeline` |

**可覆寫環境變數：**

| 變數 | 預設 | 說明 |
|------|------|------|
| `API_URL` | `http://localhost:8080` | Access API 位址 |
| `DB_DSN` | `access:access@tcp(127.0.0.1:3307)/access_control?parseTime=true` | E2E 連 MariaDB（本機對外埠 3307） |
| `DEMO_USER` / `DEMO_DOOR` | 見 §6 測試 UUID | swipe 與 DB 比對用 |
| `POLL_INTERVAL` / `POLL_TIMEOUT` | `2` / `30` | 僅 `verify-pipeline.sh` |
| `E2E_PIPELINE` | — | 設為 `1` 時才執行 `TestPipelineE2E`（`make test-e2e-pipeline` 會自動帶入） |

**失敗排查：** 事件逾時未入庫時，執行 `make logs` 查看 `aggregation-worker` 與 `kafka`；確認 `make up` 後 worker 已 Healthy。

### 3.7 與設計文件的對應

| 設計文件章節 | 實作情況 |
|--------------|----------|
| §4.1 Fast Path / Access API | 已實作 |
| §4.1.2 Redis Cache Schema | 已實作（採 `perm:denied`，非舊圖 `perm:{userId}:{doorId}`） |
| §4.1.4 Message Queue | 已實作 |
| §6.1 Normal Badge-In Flow | 流程一致 |
| §6.2 DB Down 仍開門 | Access API 不連 DB，已滿足 |
| §7 API Design | Access 端點已實作 |
| §4.1.3 Cache Invalidation | Admin API + Worker 已實作 |

---

## 4. 尚未完成項目

### 4.1 業務功能

| 項目 | 設計文件位置 | 說明 |
|------|--------------|------|
| Report API | §4.2 Slow Path | 個人/部門/稽核報表，&lt;200ms 預聚合查詢 |
| ~~Admin 封禁/解封服務~~ | §4.1.3 | **已完成**（`admin-api`） |
| ~~Cache Invalidation Worker~~ | §4.1.3 | **已完成** |
| 員工/門禁/組織主資料 | §5 ER | 已有 `employee`（demo seed）；尚無 `door`、`org_unit` 表與完整 ER |
| `CARD_NOT_FOUND` | §7 Response | API 有定義 reason，**尚未實作** cardUid 查詢 |
| Redis 掛掉時 DB fallback | §8 Resilience | 目前 Redis 不可用回 **503** |
| `pre_aggregated_reports` 更新 | §4.1.5 Worker | Worker 僅 INSERT 原始事件，未做報表預聚合 |

### 4.2 維運與雲原生

| 項目 | 說明 |
|------|------|
| Kubernetes 部署 | 目前僅 docker-compose |
| HPA | §8 換班尖峰自動擴展 |
| Grafana 進階指標 | queue lag、cache hit、report p99 | 儀表板已涵蓋 access-api；其餘指標待補 |
| Grafana 告警規則 | p99/QPS 閾值 | 儀表板有視覺閾值；Unified Alerting 未配置 |
| CI/CD、SonarQube | 評分項目，未設置 |



---

## 5. 測試情況

### 5.1 已執行且通過

| 類型 | 指令 / 方式 | 內容 | 結果 |
|------|-------------|------|------|
| 單元測試 | `make test-unit` | access-api 決策、admin-api handler、cache-invalidation-worker Redis | **通過** |
| 本機編譯 | `go build` 各模組 | access-api、admin-api、aggregation-worker、cache-invalidation-worker、badge-reader-sim | **通過** |
| Docker 建置 | `make up` | 七個服務（含 admin-api、cache-invalidation-worker） | **通過**（需 Docker Desktop） |
| 健康檢查 | `curl http://localhost:8080/health` | | `{"status":"ok"}` |
| 手動驗收 | `make demo`（`scripts/demo.sh`） | ① IN→ALLOW ② 再 IN→DENY(ANTI_PASSBACK) ③ OUT→ALLOW ④ 封禁用戶→DENY(PERMISSION_DENIED) ⑤ 查 state / door | **通過** |
| 模擬刷卡 | `make swipe` | 回傳 decision、eventId、latency | **通過**（例：latency ~17ms） |
| 鏈路驗證 | `make verify-pipeline` | 單次 swipe → 輪詢 `inout_events` 依 eventId 斷言 | **通過**（~3s 內寫入 DB） |
| Redis 整合測試 | `make test-integration` | testcontainers Redis + 反潛回 HTTP | **通過** |
| 全鏈路 E2E | `make test-e2e-pipeline` | 對 compose 堆疊：swipe → MariaDB 查表 | **通過** |
| Admin 封禁鏈路 | `make demo-ban` | ban → swipe DENY(PERMISSION_DENIED) → unban → ALLOW | **通過** |

### 5.2 Kafka → DB 驗證與 E2E（執行順序建議）

```bash
# ① 啟動完整堆疊（含 Kafka、Worker、MariaDB）
make up

# ② Shell 鏈路驗證（適合示範／手動驗收）
make verify-pipeline

# ③ Go 全鏈路 E2E（CI 或回歸用；等同 E2E_PIPELINE=1 + integration tests）
make test-e2e-pipeline

# ④ 僅 Redis 反潛回（不需 make up，但需本機 Docker 給 testcontainers）
make test-integration

# ⑤ 決策邏輯單元測試（無外部依賴）
make test-unit
```

`make test` 預設等同 `make test-unit`；完整 Kafka→DB 覆蓋需另跑 `verify-pipeline` 或 `test-e2e-pipeline`。

---

## 6. 快速啟動

```bash
# 1. 確認 Docker 運行
docker ps

# 2. 進入專案根目錄
cd "Distributed Physical Access Control System"

# 3. 啟動（build + Kafka topics + migrate + seed）
make up

# 4. 驗收
make demo              # 反潛回 + Admin 封禁示範
make demo-ban          # 封禁鏈路專用示範
make verify-pipeline   # Kafka → MariaDB
make swipe
curl http://localhost:8080/health
curl http://localhost:8081/health
make test-unit
make test-integration
make test-e2e-pipeline

# 5. 關閉並清資料
make down
```

**既有 DB volume 升級（不刪資料）：**

```bash
make migrate    # 套用 002_employee.sql
make up         # 或僅 restart 相關服務
```

### 測試用 UUID（`make seed` 後）

| 角色 | UUID |
|------|------|
| 一般員工 | `22222222-2222-2222-2222-222222222222` |
| 封禁員工（DB `is_active=0`；Redis 需 `make ban`） | `00000000-0000-0000-0000-000000000099` |
| 門禁 | `11111111-1111-1111-1111-111111111111` |

---

## 7. 已知問題與注意事項

1. **Docker 必須先啟動**：否則 `make up` 會出現 `Cannot connect to the Docker daemon`。
2. **MariaDB 對外埠為 3307**：本機若已佔用 3306，compose 已改映射；容器內服務仍用 `mariadb:3306`。
3. **Kafka 映像**：使用 `apache/kafka:3.7.0`（原設計寫 bitnami，該 tag 已不可用）。
4. **根目錄無 go.mod**：執行 Go 程式請 `cd badge-reader-sim` 等子目錄，或用 `make swipe`。
5. **封禁流程**：使用 `make ban` / Admin API；`seed-redis.sh` 不再寫入 `perm:denied`。
6. **升級 DB**：若已有 `mariadb_data` volume，請執行 `make migrate` 或 `make down -v` 後 `make up`。
7. **Kafka topic 與 Worker**：`make up` 會 `init-kafka-topics` 並重啟 `cache-invalidation-worker`，避免 topic 尚未建立時 consumer 無法訂閱。
8. **Admin API 重建**：修改 `admin-api` 後需 `docker compose up -d --build admin-api`，否則可能仍跑舊映像。
9. **設計文件 minor 差異**：Sequence 圖曾出現 `perm:{userId}:{doorId}`，實作採用較新的 **`perm:denied:{userId}`**（見 HTML §4.1.2）。

---

## 8. 建議接手優先順序

1. ~~**補驗證**~~：已完成 — `make verify-pipeline`。
2. ~~**補測試**~~：已完成 — `make test-integration`、`make test-e2e-pipeline`。
3. **依課程評分**：架構圖、ER 圖、序列圖（可從 HTML 匯出或重畫）。
4. ~~**Phase 2 Admin 封禁**~~：已完成。
5. **Phase 2 後續**：
   - Report API + `pre_aggregated_reports`
   - `CARD_NOT_FOUND`（`employee.card_uid` 查詢）
   - K8s manifests + HPA（Grafana 已完成於 compose）

---

## 9. 聯絡用關鍵檔案索引

| 若要改… | 看這裡 |
|---------|--------|
| 刷卡決策邏輯 | `access-api/internal/service/access_decision.go` |
| HTTP 路由與回應 | `access-api/internal/handler/access.go` |
| Redis 操作 | `access-api/internal/cache/redis.go` |
| Kafka 發布 | `access-api/internal/queue/kafka_producer.go` |
| DB 寫入 | `aggregation-worker/internal/repository/inout.go` |
| 封禁 API | `admin-api/internal/handler/admin.go` |
| 員工 DB | `admin-api/internal/repository/employee.go`、`migrations/002_employee.sql` |
| 權限事件發布 | `admin-api/internal/queue/kafka_producer.go` |
| Redis 封禁同步 | `cache-invalidation-worker/internal/consumer/worker.go` |
| Admin 示範腳本 | `scripts/demo-ban.sh` |
| Kafka→DB 腳本驗證 | `scripts/verify-pipeline.sh` |
| E2E / 整合測試 | `access-api/tests/integration/pipeline_e2e_test.go`、`swipe_test.go` |
| 本地環境 | `docker-compose.yml`、`Makefile` |

---

*本文件描述截至 2026-05-19 之 repo 狀態；若後續有 commit，請同步更新本文件「已完成 / 未完成 / 測試」三節。*
