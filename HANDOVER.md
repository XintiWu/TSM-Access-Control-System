# 專案交接文件 — Distributed Physical Access Control System

**交接日期：** 2026-05-19  
**範圍：** Phase 1 — Access Fast Path（刷卡機 / 門禁決策路徑）  
---

## 1. 專案目標（簡述）

為 TSMC 規模（約 9 萬員工）的實體門禁系統，實作 **刷卡當下開門決策**（Fast Path）：

- 決策延遲目標：**&lt; 50ms**（僅依賴 Redis，不查主資料庫）
- **反潛回（Anti-Passback）**：禁止連續兩次 IN 或連續兩次 OUT
- **非同步寫入**：刷卡事件經 Kafka 緩衝，再由 Worker 寫入 MariaDB
- **韌性**：主資料庫掛掉時，門仍可開（決策與事件發布不依賴 DB）

本次交接僅涵蓋架構圖中的 **Access Tier + Data Tier 之刷卡相關部分**，不含報表、管理後台、K8s 維運。

---

## 2. 目錄結構

```
Distributed Physical Access Control System/
├── docker-compose.yml          # 本地一鍵啟動全部服務
├── Makefile                    # up / down / seed / demo / swipe / test
├── README.md                   
├── migrations/
│   └── 001_inout_events.sql    # MariaDB 事件表 DDL
├── scripts/
│   ├── seed-redis.sh           # 灌測試用 Redis 資料
│   ├── demo.sh                 # 反潛回 + 封禁示範（curl + jq）
│   └── verify-pipeline.sh      # Kafka → Worker → MariaDB 鏈路驗證
├── access-api/                 # Go：Access API（Gin）
├── aggregation-worker/         # Go：Kafka → MariaDB
└── badge-reader-sim/           # Go CLI：模擬刷卡機
```

**注意：** 三個 Go 子專案各有獨立 `go.mod`，在根目錄執行 `go run` 會失敗，請 `cd` 進對應目錄或使用 `make swipe`。

---

## 3. 已完成項目

### 3.1 基礎設施

| 項目 | 說明 |
|------|------|
| Docker Compose | Redis、Kafka（apache/kafka:3.7.0）、MariaDB、access-api、aggregation-worker |
| MariaDB 初始化 | `migrations/001_inout_events.sql` 自動建立 `inout_events` 表 |
| Makefile | `make up` / `down` / `seed` / `demo` / `swipe` / `verify-pipeline` / `test-unit` / `test-integration` / `test-e2e-pipeline` |
| 測試資料腳本 | `seed-redis.sh`（支援本機無 redis-cli 時改走 `docker compose exec`） |

**埠號（本機）：**

| 服務 | 埠 |
|------|-----|
| Access API | 8080 |
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
| Kafka Topic | `inout-events`（自動建立） |
| 事件 Schema | JSON：`eventId`, `employeeId`, `doorId`, `direction`, `eventTime`, `status`, `reason`, `cardUid`, `sourceIp` |
| Producer | `access-api/internal/queue/kafka_producer.go`（含失敗重試佇列） |
| Aggregation Worker | 消費 Kafka → `INSERT IGNORE INTO inout_events`（依 eventId 去重） |

### 3.4 刷卡機模擬器（`badge-reader-sim/`）

- CLI：`go run ./cmd/sim`，支援 `--direction`、`--count`、`--interval` 壓測
- 或根目錄：`make swipe` / `make swipe DIRECTION=OUT`

### 3.5 與設計文件的對應

| 設計文件章節 | 實作情況 |
|--------------|----------|
| §4.1 Fast Path / Access API | 已實作 |
| §4.1.2 Redis Cache Schema | 已實作（採 `perm:denied`，非舊圖 `perm:{userId}:{doorId}`） |
| §4.1.4 Message Queue | 已實作 |
| §6.1 Normal Badge-In Flow | 流程一致 |
| §6.2 DB Down 仍開門 | Access API 不連 DB，已滿足 |
| §7 API Design | 三個 Access 端點已實作 |

---

## 4. 尚未完成項目

### 4.1 業務功能

| 項目 | 設計文件位置 | 說明 |
|------|--------------|------|
| Report API | §4.2 Slow Path | 個人/部門/稽核報表，&lt;200ms 預聚合查詢 |
| Admin 封禁/解封服務 | §4.1.3 | 目前僅能手動 `seed-redis.sh` 寫入 `perm:denied` |
| Cache Invalidation Worker | §4.1.3 | 消費 `permission-events`，同步封禁至 Redis |
| 員工/門禁/組織主資料 | §5 ER | 僅有 `inout_events` 表，無 `employee`、`door`、`org_unit` |
| `CARD_NOT_FOUND` | §7 Response | API 有定義 reason，**尚未實作** cardUid 查詢 |
| Redis 掛掉時 DB fallback | §8 Resilience | 目前 Redis 不可用回 **503** |
| `pre_aggregated_reports` 更新 | §4.1.5 Worker | Worker 僅 INSERT 原始事件，未做報表預聚合 |

### 4.2 維運與雲原生

| 項目 | 說明 |
|------|------|
| Kubernetes 部署 | 目前僅 docker-compose |
| HPA | §8 換班尖峰自動擴展 |
| Grafana / Prometheus 完整儀表板 | 僅有 `/metrics` 端點 |
| 告警規則 | `access_api_p99_latency_ms` 等閾值未配置 |
| CI/CD、SonarQube | 評分項目，未設置 |



---

## 5. 測試情況

### 5.1 已執行且通過

| 類型 | 指令 / 方式 | 內容 | 結果 |
|------|-------------|------|------|
| 單元測試 | `make test-unit` | `access_decision.go`：封禁、反潛回 IN/OUT、首次 IN、Redis 錯誤等 7 案例 | **通過** |
| 本機編譯 | `go build` 三個模組 | access-api、aggregation-worker、badge-reader-sim | **通過** |
| Docker 建置 | `make up` | 五個容器 Healthy + Started | **通過**（需 Docker Desktop 運行中） |
| 健康檢查 | `curl http://localhost:8080/health` | | `{"status":"ok"}` |
| 手動驗收 | `make demo`（`scripts/demo.sh`） | ① IN→ALLOW ② 再 IN→DENY(ANTI_PASSBACK) ③ OUT→ALLOW ④ 封禁用戶→DENY(PERMISSION_DENIED) ⑤ 查 state / door | **通過** |
| 模擬刷卡 | `make swipe` | 回傳 decision、eventId、latency | **通過**（例：latency ~17ms） |
| 鏈路驗證 | `make verify-pipeline` | 單次 swipe → 輪詢 `inout_events` 依 eventId 斷言 | **通過**（~3s 內寫入 DB） |
| Redis 整合測試 | `make test-integration` | testcontainers Redis + 反潛回 HTTP | **通過** |
| 全鏈路 E2E | `make test-e2e-pipeline` | 對 compose 堆疊：swipe → MariaDB 查表 | **通過** |


---

## 6. 快速啟動

```bash
# 1. 確認 Docker 運行
docker ps

# 2. 進入專案根目錄
cd "Distributed Physical Access Control System"

# 3. 啟動（含 build + seed）
make up

# 4. 驗收
make demo
make verify-pipeline
make swipe
curl http://localhost:8080/health
make test-unit
make test-integration
make test-e2e-pipeline

# 5. 關閉並清資料
make down
```

### 測試用 UUID（`make seed` 後）

| 角色 | UUID |
|------|------|
| 一般員工 | `22222222-2222-2222-2222-222222222222` |
| 封禁員工 | `00000000-0000-0000-0000-000000000099` |
| 門禁 | `11111111-1111-1111-1111-111111111111` |

---

## 7. 已知問題與注意事項

1. **Docker 必須先啟動**：否則 `make up` 會出現 `Cannot connect to the Docker daemon`。
2. **MariaDB 對外埠為 3307**：本機若已佔用 3306，compose 已改映射；容器內服務仍用 `mariadb:3306`。
3. **Kafka 映像**：使用 `apache/kafka:3.7.0`（原設計寫 bitnami，該 tag 已不可用）。
4. **根目錄無 go.mod**：執行 Go 程式請 `cd badge-reader-sim` 等子目錄，或用 `make swipe`。
5. **封禁流程為簡化版**：無 Admin API，需手動改 Redis 或擴充 `seed-redis.sh`。
6. **設計文件 minor 差異**：Sequence 圖曾出現 `perm:{userId}:{doorId}`，實作採用較新的 **`perm:denied:{userId}`**（見 HTML §4.1.2）。

---

## 8. 建議接手優先順序

1. ~~**補驗證**~~：已完成 — `make verify-pipeline`。
2. ~~**補測試**~~：已完成 — `make test-integration`、`make test-e2e-pipeline`。
3. **依課程評分**：架構圖、ER 圖、序列圖（可從 HTML 匯出或重畫）。
4. **Phase 2 功能**（擇一或並行）：
   - Report API + `pre_aggregated_reports`
   - Admin 封禁 + Cache Invalidation Worker + `permission-events` topic
   - K8s manifests + HPA + Grafana

---

## 9. 聯絡用關鍵檔案索引

| 若要改… | 看這裡 |
|---------|--------|
| 刷卡決策邏輯 | `access-api/internal/service/access_decision.go` |
| HTTP 路由與回應 | `access-api/internal/handler/access.go` |
| Redis 操作 | `access-api/internal/cache/redis.go` |
| Kafka 發布 | `access-api/internal/queue/kafka_producer.go` |
| DB 寫入 | `aggregation-worker/internal/repository/inout.go` |
| 本地環境 | `docker-compose.yml`、`Makefile` |

---

*本文件描述截至 2026-05-19 之 repo 狀態；若後續有 commit，請同步更新本文件「已完成 / 未完成 / 測試」三節。*
