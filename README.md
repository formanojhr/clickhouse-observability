# clickhouse-observability


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