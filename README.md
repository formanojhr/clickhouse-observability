This repo is a learning exercise to adopt AI agents workflow extensively to generate, modify, run and test using infrastructure like clickhouse running in docker. The goal is to attempt using AI Agent chat session to generate the scaffolding, any fxies, edits etc(with very minimal handwritten code) from an AI assisted IDE.

**Tools Used**
Cursor IDE(with different AI agents)
AI Agents using for coding testing workflow 
Built in agent chat with - chatgpt 4.0 ( as well as auto mode)
Cline https://cline.bot/ with Gemini and x-ai/grok-code-fast-1

# clickhouse-observability
**Core Purpose**
The gRPC APIs provide a single, efficient endpoint for ingesting structured log data at scale, using ClickHouse as the underlying database for fast analytics and querying.
Key API Details
**Main Service**: LogService
**RPC Method**: BatchWrite(BatchWriteRequest) → BatchWriteResponse
**Purpose**: Accepts batches of log entries and writes them to ClickHouse database
**Architecture**: Uses a batching layer for efficiency (fire-and-forget semantics)
**Log Data Structure (LogEntry message):**
ts: Timestamp (RFC3339 format)
service: Service name (used for partitioning)
level: Log level (INFO, WARN, ERROR, etc.)
msg: Log message
attrs: Key-value attributes (stored as JSON)
trace_id, span_id: Distributed tracing support
**System Architecture**
gRPC Layer: Receives log batches via BatchWrite
Batching Layer: Accumulates logs and flushes to DB based on:
Batch size (default: 500 entries)
Time window (default: 100ms)
Database Layer: ClickHouse with optimized schema:
Partitioned by month (toYYYYMM(ts))
Ordered by (service, ts)
LowCardinality columns for service/level
MergeTree engine for analytical queries
Use Cases
Microservices Logging: Collect logs from distributed services
Observability Pipelines: High-volume log ingestion for monitoring
Distributed Tracing: Store trace/span IDs with logs
Analytics: Query logs by service, time range, level, and attributes
The design prioritizes ingestion speed over immediate consistency, making it suitable for high-throughput logging scenarios where you need to collect and analyze logs from multiple services efficiently.

<!-- Run Server -->
go run ./cmd/server
<!-- Check run -->
go-log-service % sleep 5 && curl -s http://localhost:8080/live
ps aux | grep "go run" | grep -v grep
<!--should see this  -->
mramakrishnan    87446   0.0  0.0 34951584  31508 s034  S+   12:58PM   0:01.39 go run ./cmd/server
go run ./cmd/server 2>&1


<!--Access the logs query API  -->
curl -s "http://localhost:8080/v1/logs?service=orders&from=2023-01-01T00:00:00Z&to=2023-12-31T23:59:59Z&level=ERROR&limit=10"
Response 
{
  "count": 0,
  "logs": null,
  "query": {
    "from": "2023-01-01T00:00:00Z",
    "level": "ERROR", 
    "limit": 10,
    "service": "orders",
    "to": "2023-12-31T23:59:59Z",
    "user": ""
  }
}


<!--Ping API test  -->
curl -s http://localhost:8080/api/ping

<!-- Insert logs API -->

mramakrishnan@MacBook-Pro-8 go-log-service % curl -s "http://localhost:8080/v1/logs?service=orders&from=2023-01-01
T00:00:00Z&to=2023-12-31T23:59:59Z&level=ERROR&limit=10"
{"count":0,"logs":null,"query":{"from":"2023-01-01T00:00:00Z","level":"ERROR","limit":10,"service":"orders","to":"
2023-12-31T23:59:59Z","user":""}}

<!-- Query logs API  -->
curl -s "http://localhost:8080/v1/logs?from=2023-01-01T00:00:00Z&to=2023-12-31T23:59:59Z"


<!--Clickhouse client commands  -->
<!-- INSERT  -->

docker exec -it ch clickhouse-client -u default --password password --query "INSERT INTO logs (ts, service, level, msg, attrs, trace_id, span_id) VALUES (now() - INTERVAL 2 MINUTE, 'orders', 'WARN', 'Order 12346 has pending items', '{\"user\": \"jane.smith\", \"order_id\": \"12346\", \"pending_items\": 2}', 'trace-124', 'span-458')"


<!-- QUERY  -->
docker exec -it ch clickhouse-client -u default --password password --query "SELECT ts, service, level, msg, attrs, trace_id, span_id FROM logs ORDER BY ts DESC"
<!-- Query from logs table -->
 docker exec -it ch clickhouse-client -u default --password password -
-query "SELECT COUNT(*) FROM logs"

docker exec -it ch clickhouse-client -u default --password password --query "SELECT ts, service, level, msg, attrs, trace_id, span_id FROM logs WHERE service = 'orders' AND ts BETWEEN '2025-09-01 20:04:00' AND '2025-09-01 20:07:00' ORDER BY ts DESC LIMIT 10"


<!-- DESCRIBE table -->
docker exec -it ch clickhouse-client -u default --password password --query "DESCRIBE logs"


<!--Clickhouse queries  -->
SELECT ts, service, level, msg, attrs, trace_id, span_id
FROM logs
WHERE service = ? AND ts BETWEEN ? AND ?
  AND level = ? (if specified)
  AND JSONExtractString(attrs, 'user') = ? (if specified)
ORDER BY ts DESC LIMIT ?
