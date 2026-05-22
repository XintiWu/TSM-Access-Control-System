 

# Distributed Physical Access Control System

## Technical Architecture Document

**Project:** Distributed Physical Access Control System  
**Group 9:** 林采穎 · 陳沐頤 · 吳昕醍 · 胡仕杰  
**Version:** 1.0

-----

## Table of Contents

1. [System Overview](#1-system-overview)
1. [Architecture Design Philosophy](#2-architecture-design-philosophy)
1. [High-Level System Architecture](#3-high-level-system-architecture)
1. [Component Breakdown](#4-component-breakdown)

- 4.1 [Access Control Path (Fast Path)](#41-access-control-path-fast-path)
- 4.2 [Reporting Path (Slow Path)](#42-reporting-path-slow-path)
- 4.3 [Cache Invalidation](#43-cache-invalidation)

1. [Data Model](#5-data-model)
1. [Sequence Diagrams](#6-sequence-diagrams)

- 6.1 [Normal Badge-In Flow](#61-normal-badge-in-flow)
- 6.2 [DB Down / Resilience Flow](#62-db-down--resilience-flow)
- 6.3 [Manager Report Query Flow](#63-manager-report-query-flow)

1. [API Design](#7-api-design)
1. [Non-Functional Requirements Implementation](#8-non-functional-requirements-implementation)
1. [Infrastructure & Deployment](#9-infrastructure--deployment)

-----

## 1. System Overview

The Distributed Physical Access Control System records employee badge-in/out events and generates hierarchical attendance reports. The core architectural challenge is a **write/read conflict**:

- **Doors must open in milliseconds** (write-heavy, latency-critical)
- **Reports are complex hierarchical aggregations** (read-heavy, accuracy-critical)

The solution is a **decoupled, event-driven architecture** that separates the Access Decision path from the Reporting pipeline.

-----

## 2. Architecture Design Overview

|Concern           |Design Decision                                 |Rationale                                         |
|------------------|------------------------------------------------|--------------------------------------------------|
|Door open latency |Stateless Access API + Redis Cache              |Avoid DB round-trip on hot path                   |
|Peak hour traffic |Message Queue as buffer                         |Requests don’t hit backend directly               |
|DB unavailability |Queue buffers events until DB recovers          |Resilience requirement                            |
|Report performance|Pre-aggregated materialized views               |Sub-200ms report rendering                        |
|Anti-passback     |Redis cache tracks entry/exit state per user    |Millisecond lookup, no DB dependency              |
|Permission model  |Only DENY entries stored in Redis               |Most employees allowed by default; absence = ALLOW|
|Cache invalidation|Ban events via Kafka → Cache Invalidation Worker|Near-instant ban enforcement without TTL delay    |
|Observability     |Grafana + Prometheus                            |Visualize Shift Change spikes                     |

-----

## 3. Overall System Architecture

```mermaid
graph TB
    subgraph Clients
        E[👤 Employee<br/>Badge Reader]
        M[💼 Manager<br/>Web Dashboard]
        ADM[🔒 Admin<br/>Ban/Unban User]
    end

    subgraph Fast Path - Access Control
        API[Access API<br/>Stateless RESTful<br/>sub-50ms]
        CACHE[(Redis Cache<br/>Denied Users<br/>Anti-Passback)]
    end

    subgraph Buffer Layer
        MQ[Message Queue<br/>Kafka<br/>Topic: inout-events]
        PMQ[Message Queue<br/>Kafka<br/>Topic: permission-events]
    end

    subgraph Slow Path - Reporting
        RAPI[Report API<br/>sub-200ms render]
        AGG[Aggregation Worker<br/>Pre-compute reports]
        RCACHE[(Report Cache<br/>Pre-aggregated<br/>Results)]
    end

    subgraph Cache Invalidation
        CW[Cache Invalidation Worker]
    end

    subgraph Persistence
        DB[(ClickHouse<br/>InOut Events + Org Tree)]
    end

    subgraph Observability
        PROM[Prometheus<br/>Metrics Scraper]
        GRAF[Grafana<br/>Dashboard]
    end

    subgraph Infrastructure
        HPA[Kubernetes HPA<br/>Auto-scaling]
    end

    E -->|Badge swipe| API
    API -->|Check denied list| CACHE
    API -->|Publish event| MQ
    API -->|Open/Deny door| E

    ADM -->|Ban/Unban user| DB
    ADM -->|Publish BanEvent| PMQ
    PMQ -->|Consume| CW
    CW -->|SET / DELETE perm:denied| CACHE

    MQ -->|Async consume| AGG
    AGG -->|Durable write| DB
    AGG -->|Pre-compute| RCACHE

    M -->|Query report| RAPI
    RAPI -->|Read pre-computed| RCACHE
    RAPI -->|Fallback read| DB

    API --> PROM
    RAPI --> PROM
    MQ --> PROM
    PROM --> GRAF

    HPA -.->|Scale out/in| API
    HPA -.->|Scale out/in| RAPI
```

-----

## 4. Component Breakdown

### 4.1 Access Control Path (Fast Path)

The Fast Path handles the physical door open/deny decision. Latency target: **< 50ms end-to-end**.

#### 4.1.1 Access API

```mermaid
flowchart TD
    A[Badge Swipe Request<br/>POST /access/swipe] --> B{Check Redis:<br/>perm:denied:userId exists?}
    B -->|Key EXISTS = DENY| Z[Return: DENY<br/>Door stays closed]
    B -->|Key ABSENT = ALLOW| C{Check Redis:<br/>Anti-Passback State?}

    C -->|State = OUT or NONE<br/>and direction = IN| E[Return: ALLOW<br/>Open door]
    C -->|State = IN<br/>and direction = IN| F[Return: DENY<br/>Anti-Passback violation]
    C -->|State = IN<br/>and direction = OUT| E

    E --> G[Publish InOutEvent<br/>to Message Queue]
    Z --> G2[Publish FailedEvent<br/>to Message Queue]
    F --> G2
```

**Key design decisions:**

- The API is **stateless** — all state lives in Redis, enabling horizontal scaling
- **Only denied users are cached** — absence of a key means ALLOW; DB is never on the hot path for normal employees
- **Anti-passback** is enforced via a Redis key per user: `passback:{userId}` = `IN` | `OUT`
- All events (success and failure) are published to the queue for durability and analytics

#### 4.1.2 Redis Cache Schema

|Key Pattern           |Value               |TTL                 |Purpose                            |
|----------------------|--------------------|--------------------|-----------------------------------|
|`perm:denied:{userId}`|`DENY`              |24h                 |Denied user cache (absence = ALLOW)|
|`passback:{userId}`   |`IN` / `OUT`        |24h (reset midnight)|Anti-passback state                |
|`door:status:{doorId}`|`ONLINE` / `OFFLINE`|30s heartbeat       |Door health                        |

**Permission cache design:** Only **denied** users are stored in cache. If a key is absent, the user is considered allowed. This simplifies the default case — most employees have access — and means a cache miss no longer requires a DB lookup for normal employees.

#### 4.1.3 Cache Invalidation

Storing only denied users introduces a **cache inconsistency window**: if a user is banned in the DB but their `DENY` key has not yet been written to Redis, they will still be allowed entry. To address this, ban events are propagated via a dedicated Kafka topic consumed by a Cache Invalidation Worker:

```mermaid
sequenceDiagram
    participant ADM as Admin Service
    participant D as ClickHouse
    participant Q as Message Queue
    participant CW as Cache Invalidation Worker
    participant R as Redis Cache

    ADM->>D: UPDATE employee SET is_active = false
    ADM->>Q: PUBLISH BanEvent {userId, action: BAN}
    Note over Q: Topic: permission-events

    Q->>CW: CONSUME BanEvent
    CW->>R: SET perm:denied:{userId} = DENY (TTL: 24h)
    Note over R: Next swipe by this user → immediate DENY
```

**Key properties:**

- Ban takes effect on the **next swipe** after the worker processes the event — typically within seconds
- `permission-events` is a separate Kafka topic from `inout-events`, so ban propagation is never delayed by swipe traffic
- On **unban**, the worker deletes the `perm:denied:{userId}` key — user is immediately allowed again

#### 4.1.4 Message Queue (Buffer Layer)

The queue is the critical component that enables **resilience** and **peak-load handling**:

```mermaid
graph LR
    API[Access API] -->|Publish| Q[(Message Queue <br> Topic: inout-events)]
    Q -->|Consume| W1[Aggregation Worker 1]
    Q -->|Consume| W2[Aggregation Worker 2]
    W1 -->|Write| DB[(ClickHouse)]
    W2 -->|Write| DB
```

- **When DB is down:** Events accumulate in the queue. The door still opens (access decision was already made from cache). Once DB recovers, workers drain the queue and persist all events.
- **During peak hours (shift change):** The queue absorbs traffic spikes. Workers consume at a steady rate, preventing DB connection pool exhaustion.

-----

### 4.2 Reporting Path (Slow Path)

The Slow Path serves manager dashboards and attendance reports. Latency target: **< 200ms render time**.

#### 4.2.1 Aggregation Worker

The worker consumes events from the queue and does two things:

1. **Durable write** — persists the raw `InOutEvent` to ClickHouse
1. **Pre-aggregation** — incrementally updates materialized report caches

```mermaid
flowchart LR
    Q[(Message Queue)] -->|Consume InOutEvent| W[Aggregation Worker]
    W -->|INSERT raw event| RAW[(inout_events table)]
    W -->|UPDATE counters| AGG[(pre_aggregated_reports table)]
    W -->|Invalidate| RC[(Report Cache <br> Redis)]
```


#### 4.2.2 Hierarchical Report Access (Permission Control)

Managers automatically see data for their team and all sub-teams. This is enforced via the **Organizational Tree** stored in the DB:

```mermaid
graph TD
    CEO[CEO / CFO <br> See: entire org]
    VP[VP / Director <br> See: their division]
    MGR[Team Manager <br> See: their team]
    EMP[Employee <br> See: own records only]

    CEO --> VP --> MGR --> EMP
```

When a report is requested, the Report API resolves the requesting user’s position in the org tree and filters data to their subtree.

-----

## 5. Data Model

```mermaid
erDiagram
    EMPLOYEE {
        uuid id PK
        string name
        string card_uid
        boolean is_active
        uuid org_unit_id FK
    }

    ORG_UNIT {
        uuid id PK
        string name
        uuid parent_id FK
        int depth
        string materialized_path
    }

    DOOR {
        uuid id PK
        string location
        string building
        boolean is_active
    }

    INOUT_EVENT {
        uuid id PK
        uuid employee_id FK
        uuid door_id FK
        enum direction
        timestamp event_time
        enum status
        string source_ip
    }

    PRE_AGGREGATED_REPORT {
        uuid id PK
        uuid org_unit_id FK
        date report_date
        int total_entries
        int total_exits
        float avg_hours
        timestamp computed_at
    }

    EMPLOYEE ||--o{ INOUT_EVENT : "generates"
    DOOR ||--o{ INOUT_EVENT : "records"
    ORG_UNIT ||--o{ EMPLOYEE : "contains"
    ORG_UNIT ||--o{ PRE_AGGREGATED_REPORT : "summarizes"
    ORG_UNIT ||--o| ORG_UNIT : "parent of"
```

**Notes on `ORG_UNIT`:**

- `materialized_path` stores the full ancestry path (e.g., `/root/div-a/team-3/`) enabling fast subtree queries without recursive joins
- `depth` field enables level-based queries (e.g., “show all teams under this VP”)

-----

## 6. Sequence Diagrams

### 6.1 Normal Badge-In Flow

```mermaid
sequenceDiagram
    participant B as Badge Reader
    participant A as Access API
    participant R as Redis Cache
    participant Q as Message Queue
    participant W as Aggregation Worker
    participant D as ClickHouse

    B->>A: POST /access/swipe {userId, doorId, direction: IN}
    A->>R: GET perm:{userId}:{doorId}
    R-->>A: ALLOW (cache hit)
    A->>R: GET passback:{userId}
    R-->>A: OUT (last state)
    A->>R: SET passback:{userId} = IN
    A->>Q: PUBLISH InOutEvent {userId, doorId, direction, timestamp}
    A-->>B: 200 OK {decision: ALLOW} ← door opens
    Note over A,B: < 50ms total

    Q->>W: CONSUME InOutEvent
    W->>D: INSERT INTO inout_events
    W->>D: UPDATE pre_aggregated_reports
    Note over W,D: Async, best-effort, ~seconds later
```

### 6.2 DB Down / Resilience Flow

```mermaid
sequenceDiagram
    participant B as Badge Reader
    participant A as Access API
    participant R as Redis Cache
    participant Q as Message Queue
    participant W as Aggregation Worker
    participant D as ClickHouse

    Note over D: ⚠️ Database is DOWN

    B->>A: POST /access/swipe
    A->>R: GET perm:{userId}:{doorId}
    R-->>A: ALLOW (still cached)
    A->>R: SET passback:{userId} = IN
    A->>Q: PUBLISH InOutEvent
    A-->>B: 200 OK — door opens ✅
    Note over Q: Events accumulate in queue

    Note over D: ✅ Database RECOVERS

    Q->>W: CONSUME buffered events (backlog)
    W->>D: INSERT all pending events
    Note over W: All events eventually consistent
```

### 6.3 Manager Report Query Flow

```mermaid
sequenceDiagram
    participant M as Manager Browser
    participant RA as Report API
    participant RC as Report Cache (Redis)
    participant D as ClickHouse

    M->>RA: GET /reports/department?orgUnitId=X&month=2026-04
    RA->>RA: Resolve org subtree for requesting user
    RA->>RC: GET report:X:2026-04
    RC-->>RA: HIT — return pre-aggregated JSON
    RA-->>M: 200 OK {report data} ← render only
    Note over RA,M: < 200ms

    alt Cache MISS
        RA->>D: SELECT from pre_aggregated_reports WHERE org path LIKE '/X/%'
        D-->>RA: Aggregated rows
        RA->>RC: SET report:X:2026-04 (TTL: 5min)
        RA-->>M: 200 OK {report data}
    end
```

-----

## 7. API Design

### Access API Endpoints

|Method|Path                             |Description                           |Latency Target|
|------|---------------------------------|--------------------------------------|--------------|
|`POST`|`/access/swipe`                  |Process badge swipe, return ALLOW/DENY|< 50ms        |
|`GET` |`/access/door/{doorId}/status`   |Get door online/offline status        |< 100ms       |
|`GET` |`/access/employee/{userId}/state`|Get current IN/OUT state              |< 100ms       |

**POST /access/swipe — Request:**

```json
{
  "userId": "uuid",
  "doorId": "uuid",
  "direction": "IN | OUT",
  "cardUid": "string",
  "timestamp": "ISO8601"
}
```

**POST /access/swipe — Response:**

```json
{
  "decision": "ALLOW | DENY",
  "reason": "ANTI_PASSBACK | PERMISSION_DENIED | CARD_NOT_FOUND | null",
  "eventId": "uuid"
}
```

-----

### Report API Endpoints

|Method|Path                 |Description                      |Latency Target|
|------|---------------------|---------------------------------|--------------|
|`GET` |`/reports/personal`  |Employee’s own monthly attendance|< 200ms       |
|`GET` |`/reports/department`|Manager’s team/org-unit report   |< 200ms       |
|`GET` |`/reports/audit`     |Full raw event log (auditors)    |< 500ms       |
|`GET` |`/reports/export`    |Download PDF / CSV               |async         |

**GET /reports/department — Query Params:**

|Param        |Type  |Required|Description                                           |
|-------------|------|--------|------------------------------------------------------|
|`orgUnitId`  |uuid  |yes     |Target org unit (auto-filtered to requester’s subtree)|
|`startDate`  |date  |yes     |Report start date                                     |
|`endDate`    |date  |yes     |Report end date                                       |
|`granularity`|`daily|weekly  |monthly`                                              |

-----

## 8. Non-Functional Requirements Implementation

### Elastic Scalability (HPA)

```mermaid
graph LR
    HPA[Kubernetes HPA] -->|Monitor CPU/QPS| M[Prometheus Metrics]
    M -->|QPS spike detected| HPA
    HPA -->|Scale OUT| P1[Access API Pod 1]
    HPA -->|Scale OUT| P2[Access API Pod 2]
    HPA -->|Scale OUT| P3[Access API Pod 3]
    P1 & P2 & P3 -->|All read from| R[(Redis)]
    P1 & P2 & P3 -->|All publish to| Q[(Message Queue)]
```

Because the Access API is **stateless** (all state is in Redis), scaling out is safe — any pod can handle any request.

-----

### Resilience

|Failure Scenario    |Behavior                                                        |Recovery                         |
|--------------------|----------------------------------------------------------------|---------------------------------|
|DB down             |Queue buffers events; doors still open from Redis cache         |DB recovers → workers drain queue|
|Redis down          |Access API falls back to DB permission check (slower path)      |Redis restarts → cache warms up  |
|Message Queue down  |Access API returns event ID and logs locally; retry on reconnect|Queue recovers → events replayed |
|Access API pod crash|Kubernetes restarts pod; HPA spins up replacement               |< 30s recovery                   |

-----

### Observability (Grafana Dashboard)

Key metrics to monitor, especially during **Shift Change** (peak period):

|Metric                     |Description                             |Alert Threshold|
|---------------------------|----------------------------------------|---------------|
|`access_api_qps`           |Requests per second on Access API       |> 1000 rps     |
|`access_api_p99_latency_ms`|99th percentile door decision latency   |> 50ms         |
|`queue_consumer_lag`       |Events in queue not yet written to DB   |> 10,000       |
|`redis_cache_hit_rate`     |% of permission checks served from cache|< 90%          |
|`db_connection_pool_usage` |Active DB connections                   |> 80%          |
|`report_api_p99_latency_ms`|99th percentile report render time      |> 200ms        |

-----

## 9. Infrastructure & Deployment

```mermaid
graph TB
    subgraph Kubernetes Cluster
        subgraph Access Tier
            A1[Access API Pod]
            A2[Access API Pod]
            A3[Access API Pod]
        end
        subgraph Report Tier
            R1[Report API Pod]
            R2[Report API Pod]
        end
        subgraph Worker Tier
            W1[Aggregation Worker]
            W2[Aggregation Worker]
        end
    end

    subgraph Data Tier
        REDIS[(Redis\nCache + Anti-Passback)]
        KAFKA[(Message Queue\nKafka)]
        PG[(ClickHouse\nPrimary DB)]
        RCACHE[(Redis\nReport Cache)]
    end

    subgraph Observability
        PROM[Prometheus]
        GRAF[Grafana]
    end

    LB[Load Balancer] --> A1 & A2 & A3
    LB --> R1 & R2
    A1 & A2 & A3 --> REDIS & KAFKA
    W1 & W2 --> PG & RCACHE
    KAFKA --> W1 & W2
    R1 & R2 --> RCACHE & PG
    PROM --> GRAF
```

**Key infrastructure notes:**

- Access API, Report API, and Aggregation Workers are independently scalable deployments
- Redis is used for two separate concerns (permission/passback cache AND report cache) — can be the same cluster with separate key namespaces
- Message Queue (Kafka recommended) provides durability via log-based storage, ensuring no events are lost even during consumer downtime
- ClickHouse can be sharded by `org_unit_id` or by time range for large-scale deployments, as noted in meeting notes

-----

*Document generated for TSMC Cloud Native 2026 — Group 9*

