# Archived MariaDB migrations

These SQL files were used when MariaDB was the primary store. The project now uses **ClickHouse only**.

- Schema and demo data: [`clickhouse/init.sql`](../clickhouse/init.sql), [`clickhouse/seed.sql`](../clickhouse/seed.sql)
- Apply seed after `make up`: `make seed-ch`

Files here are kept for historical reference only.
