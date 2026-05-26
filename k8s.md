# GCP 託管型監控服務（GMP + Grafana）上雲實施方案

本方案旨在將分散式物理門禁系統（Distributed Physical Access Control System）的觀測指標（Metrics）與監控面板（Grafana/Prometheus）搬遷至 **Google Cloud Platform (GCP)** 託管型服務。

我們將採用 **Google Cloud Managed Service for Prometheus (GMP)** 作為時序指標收集與儲存引擎，並搭配 **Google Cloud Managed Service for Grafana**（或 Grafana Cloud）作為視覺化與告警中心。此方案能大幅降低運維成本，提供全球規模的高可用性與資料持久性。

---

## 🏗️ 系統架構對比

| 組件 | 本地開發環境 (Docker Compose) | GCP 託管型雲端生產環境 (GKE + GMP + Managed Grafana) |
| :--- | :--- | :--- |
| **API 指標端點** | 暴露於容器內 8080 / 8082 的 /metrics | 部署於 GKE 中，利用 Pod 內部網路隔離，僅對 GKE 內部開放。 |
| **指標搜集器** | 自行運行的 Prometheus 容器 (定期 Scrape) | GKE 內建 of **GMP Managed Collector** (自動注入的輕量化 Agent)。 |
| **時序資料庫** | Prometheus 容器內的本地硬碟儲存 (TSDB) | **Monarch (GCP 全託管時序資料庫)**，無限水平擴展、雙區備份。 |
| **視覺化面板** | 自行運行的 Grafana 容器 (`localhost:3001`) | **Google Cloud Managed Service for Grafana**，整合 GCP IAM 安全登入。 |
| **告警轉發** | Grafana 本地 Alerting 轉發至 Slack | Grafana Alerting / GCP Cloud Monitoring 整合 Slack 與 PagerDuty。 |

---

## 🛠️ Step-by-Step 上雲實施步驟

### 📋 事前準備 (Prerequisites)
1. 準備一個運行中的 **GKE 叢集**（建議啟用 Workload Identity 以保障 IAM 安全）。
2. 本地終端機已安裝並配置 **Google Cloud CLI (`gcloud`)** 與 **`kubectl`**。
3. 確保 access-api 和 report-api 已經成功部署在 GKE 的 access-control Namespace 中。

---

### Step 1: 在 GKE 叢集啟用全託管 Prometheus (GMP)

Google Cloud Managed Service for Prometheus (GMP) 提供全託管的收集器。您無須手動配置 Prometheus Deployment 與儲存硬碟。

執行以下 gcloud 指令，在現有的 GKE 叢集中啟用 GMP：

bash
# 請替換您的叢集名稱與區域/地區
gcloud container clusters update <YOUR_CLUSTER_NAME> \
    --region <YOUR_CLUSTER_REGION> \
    --enable-managed-prometheus

> [!NOTE]
> 啟用此功能後，GKE 會自動在叢集中部署輕量級的 gmp-operator 和搜集器 Pod，並準備好接收 PodMonitoring 資源配置。

---

### Step 2: 部署 PodMonitoring 自定義資源 (CRD)

GMP 不使用傳統 Prometheus 的 prometheus.yml 設定檔，而是使用 Kubernetes-native 的 **`PodMonitoring`** 資源來宣告哪些 Pod 需要被抓取指標。

請建立以下兩個 YAML 檔案並部署至叢集：

#### 1. 監控 access-api 指標
建立 `k8s/monitoring/access-api-monitoring.yaml`：

yaml
apiVersion: monitoring.gke.io/v1
kind: PodMonitoring
metadata:
  name: access-api-monitoring
  namespace: access-control
spec:
  selector:
    matchLabels:
      app: access-api
  endpoints:
  - port: 8080
    path: /metrics
    interval: 15s

#### 2. 監控 report-api 指標
建立 `k8s/monitoring/report-api-monitoring.yaml`：

yaml
apiVersion: monitoring.gke.io/v1
kind: PodMonitoring
metadata:
  name: report-api-monitoring
  namespace: access-control
spec:
  selector:
    matchLabels:
      app: report-api
  endpoints:
  - port: 8082
    path: /metrics
    interval: 15s

#### 部署至 GKE：
bash
kubectl apply -f k8s/monitoring/access-api-monitoring.yaml
kubectl apply -f k8s/monitoring/report-api-monitoring.yaml

> [!TIP]
> 部署完成後，GMP 的 Managed Collector 會自動發現帶有 app: access-api 與 app: report-api 標籤的 Pod，並每 15 秒抓取一次 /metrics 指標，直接安全地寫入 Google Cloud Monarch 儲存庫。

---

### Step 3: 建立 Google Cloud Managed Service for Grafana

我們將在 GCP 上啟用託管型 Grafana，以確保與 GCP 的監控權限（IAM）完美整合。

1. 登入 **GCP Console**。
2. 在搜尋欄輸入 **"Grafana"**，選擇 **Managed Service for Grafana**。
3. 點選 **"Create Instance" (建立執行個體)**。
4. 設定基本參數：
   * **Instance Name:** access-control-grafana
   * **Region:** 選擇與您的 GKE 叢集相同的區域（例如 `asia-east1`）以將網路延遲降到最低。
   * **Access Control:** 啟用 **GCP Single Sign-On (SSO)**，限制只有特定 Google Workspace 帳號或 GCP 專案成員可以登入。

---

### Step 4: 配置 Grafana IAM 權限與 Data Sources

為了讓託管型 Grafana 能夠向 GMP 和 ClickHouse 讀取數據，我們需要為其配置正確的權限。

#### 1. 配置 GCP 權限（讀取 GMP 指標）
1. 前往 GCP 控制台的 **IAM & Admin (IAM 與管理)** 頁面。
2. 找到系統為 Grafana 自動生成的 Service Account（格式通常為 `service-<PROJECT_NUMBER>@gcp-grafana.iam.gserviceaccount.com`）。
3. 為該 Service Account 新增以下角色：
   * **Monitoring Viewer (`roles/monitoring.viewer`)**：允許 Grafana 讀取 GMP 收集的時序指標。

#### 2. 在 Grafana 中新增 Prometheus (GMP) 資料來源
1. 登入已建立的 Grafana 控制台。
2. 點選 **Connections -> Data sources -> Add data source**。
3. 選擇 **Prometheus**。
4. 設定參數：
   * **Name:** Google Cloud Managed Prometheus
   * **URL:** http://localhost:9090 (託管型 Grafana 會自動將查詢代理至 GCP GMP 後端，在配置精靈中直接啟用 **"Google Cloud Authentication"** 即可)。
   * **Auth:** 勾選 **"Google Cloud Default Credentials"**。
5. 點選 **Save & test**，看到綠色的 Data source is working 即可！

#### 3. 新增 ClickHouse 資料來源
因為專案的 **Access Analytics（門禁熱圖、平均工時）** 存放在 ClickHouse，Grafana 必須與其建立連線。
1. 在 Grafana 中點選 **Add data source**，搜尋並安裝 **ClickHouse** 插件（專案本機已整合此插件）。
2. 設定參數：
   * **Name:** ClickHouse-Analytics
   * **Server Address:** 填入您雲端 ClickHouse 的連接網址（若 ClickHouse 部署在 GKE 內，可填寫內網 DNS：`clickhouse.access-control.svc.cluster.local`）。
   * **Server Port:** 8123 (HTTP 介面)。
   * **Username:** default
   * **Password:** 填入 app-secrets 中設定的密碼。
3. 點選 **Save & test** 測試連線。

---

### Step 5: 匯入儀表板與設定 Slack 告警

#### 1. 匯入本地儀表板
我們可以直接將專案中 monitoring/grafana/dashboards/ 底下的 JSON 檔案匯入至雲端 Grafana：
* **`shift-change-monitor.json`**：展示門禁刷卡 QPS、p99 延遲、允許/拒絕趨勢。
* **`access-analytics.json`**：展示 ClickHouse 讀取的門禁熱圖、遲到率與異常反潛回 (Anti-Passback) 警報。

*匯入步驟：在 Grafana 左側選單點選 **Dashboards -> New -> Import**，並上傳上述 JSON 內容即可。*

#### 2. 設定 Anti-Passback 異常 Slack 告警
當系統偵測到有人意圖尾隨或重複刷卡（反潛回異常），我們需要立刻通知安全團隊。

1. **設定聯絡點 (Contact Point)：**
   * 在 Grafana 點選 **Alerting -> Contact points**。
   * 新增一個聯絡點，命名為 `security-slack`。
   * Integration 選擇 **Slack**，並貼上您公司的 Slack Webhook URL。
2. **建立告警規則 (Alerting Rule)：**
   * 前往 **Alerting -> Alert rules -> Create rule**。
   * **指標查詢 (Query):** 選擇 Google Cloud Managed Prometheus 作為資料來源，並輸入公式：
     
promql
     report_passback_deny_1m_max >= 50
     

   * **時間條件 (Evaluation):** 設定每 15s 評估一次，持續時間為 `0m`（一旦超過 50 次立即發報）。
   * **通知關聯 (Notification):** 將此 Rule 指向剛才建立的 `security-slack`。

---

## 🔒 安全性防護與實施建議 (Security Best Practices)

> [!WARNING]
> 時序指標與門禁數據攸關企業安全，請務必遵循以下規範：

1. **網路隔離 (Network Security)：**
   * 絕不要將 API 的 /metrics 端口映射到 GKE Ingress 的 Public 路由上。
   * PodMonitoring 是透過 GKE 內部 Pod 網路進行指標抓取，外部網際網路無法直接存取，這提供了天然的安全邊界。
2. **最小特權原則 (Least Privilege)：**
   * 授權給 Grafana 的 Service Account 僅能擁有 roles/monitoring.viewer 唯讀權限，切勿給予過大的 Editor 或 Owner 權限。
3. **敏感資訊保護 (Secrets Management)：**
   * ClickHouse 的連線密碼，切勿直接寫在 Grafana 設定檔的 Git 提交中。應透過 GKE Secret 注入，或在 Grafana 網頁端介面手動輸入保存。