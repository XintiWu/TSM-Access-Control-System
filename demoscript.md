# TSMC Distributed Physical Access Control System (PACS) - Demo Cheat Sheet

本文件整理了 TSMC PACS 系統的各個演示情境 (Cue) 與對應的測試/驗證腳本，協助你在演示或進行效能測試時快速調用。

## 💡 如何開啟與運行指令管理器？

你可以直接執行統一的指令管理器，透過互動式選單操作所有腳本：
```bash
./scripts/run.sh
```

---

## 📋 演示情境 (Cue) 與對應腳本對照表

| 演示情境 (Cue) | 選單代號 | 獨立執行指令 (推薦) | Makefile 指令 | 腳本功能說明 |
| :--- | :---: | :--- | :--- | :--- |
| **1. 基礎刷卡與重複刷卡防護** | `1` | `bash scripts/demo/demo.sh` | `make swipe-demo` | 模擬 IN/OUT 正常刷卡及反潛回 (Anti-Passback) 拒絕機制。 |
| **2. 員工停權、快取失效同步** | `2` | `bash scripts/demo/demo-ban.sh` | `make demo-ban` | 模擬管理端 Ban 掉員工，經由 Kafka 廣播使 access-api 快取秒級失效並拒絕進入。 |
| **3. 產生反潛回警報流量 (Grafana)**| `3` | `bash scripts/demo/demo-passback-alert.sh` | `make demo-passback-alert` | 短時間送出 55 次違規刷卡，觸發 Prometheus 指標，用以演示 Grafana/Slack 警報。 |
| **4. 報表權限過濾與檔案匯出** | `4` | `bash scripts/demo/demo-report.sh` | `make demo-report` | 演示不同主管權限的組織樹報表、包含 IP 的稽核日誌，並匯出 CSV 與 PDF。 |
| **5. 大人流量與全系統完整演示** | `5` | `bash scripts/demo/demo-full-flow.sh` | `make demo-full` | **全套完整演示**：產生大流量刷卡 $\rightarrow$ 經過 Kafka $\rightarrow$ 聚合寫入 ClickHouse $\rightarrow$ 產出 PDF。 |
| **6. 數據流端到端持久化確認** | `6` | `bash scripts/demo/verify-pipeline.sh` | `make verify-pipeline` | 精準驗證一次刷卡是否能在 30 秒內經由 Kafka 成功寫入 ClickHouse 資料庫。 |
| **7. 系統效能 SLA 目標驗證** | `7` | `bash scripts/demo/verify-performance.sh` | `make verify-performance` | 驗證 Fast Path (刷卡 < 50ms) 與 Slow Path (報表 < 200ms) 的 SLA 效能指標。 |
| **8. 報表 API 壓測與延遲統計** | `8` | `bash scripts/demo/benchmark-report-api.sh` | `make benchmark-report-api` | 對多個報表 API 端點進行連續壓測，並輸出 p50, p95, p99 的延遲分佈。 |
| **9. Redis 快取機制與失效驗證** | `9` | `bash scripts/demo/test-report-cache.sh` | `make test-report-cache` | 驗證報表系統的快取命中、快取未命中、快取 TTL，以及清除快取後的動態更新。 |
| **10. 初始化/重新灌入測試卡號**| `10`| `bash scripts/utils/seed-redis.sh` | `make seed` | 快速重新初始化 Redis 中的閘機狀態、員工卡號綁定等 Demo 基礎資料。 |
| **11. 生成 9 萬名虛擬員工資料** | `11`| `bash scripts/utils/gen-load-users-sql.sh`| `make seed-load-users` | 產生 90,000 名虛擬員工的 SQL 語句，並匯入 ClickHouse 用於超高負載演示。 |

---

## 🎯 演示黃金三大極簡組合 (推薦)

如果演示時間有限，建議專注於以下三個選項，就能展現最完整的核心架構：

1. **選項 `2` (員工停權)**：展示「停權員工 $\rightarrow$ Redis 快取失效 $\rightarrow$ 秒級拒絕」的分散式高併發即時同步。
2. **選項 `5` (全系統整合)**：展示「大流量刷卡 $\rightarrow$ Kafka $\rightarrow$ ClickHouse $\rightarrow$ 產生 PDF 報表」的完整微服務串接。
3. **選項 `3` (反潛回告警)**：展示「連續刻意違規 $\rightarrow$ 警報觸發 $\rightarrow$ 儀表板動態跳動」的 Prometheus/Grafana 系統監控。
