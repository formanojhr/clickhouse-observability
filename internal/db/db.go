package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2" // ClickHouse driver for database/sql
)

type DB struct{ *sql.DB }

// DSN example (host):  clickhouse://default:@localhost:9000/logs?dial_timeout=10s&read_timeout=5s&compression=lz4
// DSN example (docker): clickhouse://default:@ch:9000/logs?dial_timeout=10s&read_timeout=5s&compression=lz4
func Open(ctx context.Context, dsn string) (*DB, error) {
	sqlDB, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(50)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := initSchema(ctx, sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return &DB{sqlDB}, nil
}

func initSchema(ctx context.Context, db *sql.DB) error {
	ddl := `
CREATE TABLE IF NOT EXISTS logs (
  ts        DateTime64(3, 'UTC'),
  service   LowCardinality(String),
  level     LowCardinality(String),
  msg       String,
  attrs     String,
  trace_id  String,
  span_id   String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (service, ts)
SETTINGS index_granularity = 8192
`
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return err
	}

	// Optional: set retention TTL via env (e.g., RETENTION_DAYS=30)
	if d := os.Getenv("RETENTION_DAYS"); d != "" {
		if days, err := strconv.Atoi(d); err == nil && days > 0 {
			_, _ = db.ExecContext(ctx,
				fmt.Sprintf(`ALTER TABLE logs MODIFY TTL ts + INTERVAL %d DAY DELETE`, days))
		}
	}
	return nil
}

type Log struct {
	Ts      time.Time
	Service string
	Level   string
	Msg     string
	Attrs   map[string]string
	TraceID string
	SpanID  string
}

// QueryLogs retrieves logs based on the specified filters
func (d *DB) QueryLogs(ctx context.Context, service, level, user string, from, to time.Time, limit int) ([]Log, error) {
	query := `
		SELECT ts, service, level, msg, attrs, trace_id, span_id
		FROM logs
		WHERE service = ? AND ts BETWEEN ? AND ?`
	
	args := []interface{}{service, from, to}
	
	if level != "" {
		query += ` AND level = ?`
		args = append(args, level)
	}
	
	if user != "" {
		query += ` AND JSONExtractString(attrs, 'user') = ?`
		args = append(args, user)
	}
	
	query += ` ORDER BY ts DESC LIMIT ?`
	args = append(args, limit)
	
	log.Printf("[TRACE] Executing ClickHouse query: %s with args: %v", query, args)
	
	start := time.Now()
	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("[ERROR] ClickHouse query failed: %v", err)
		return nil, err
	}
	defer rows.Close()
	
	log.Printf("[TRACE] ClickHouse query executed in %v", time.Since(start))
	
	var logs []Log
	for rows.Next() {
		var l Log
		var attrsStr string
		if err := rows.Scan(&l.Ts, &l.Service, &l.Level, &l.Msg, &attrsStr, &l.TraceID, &l.SpanID); err != nil {
			log.Printf("[ERROR] Failed to scan row: %v", err)
			return nil, err
		}
		
		// Parse attrs JSON string back to map
		if attrsStr != "" {
			if err := json.Unmarshal([]byte(attrsStr), &l.Attrs); err != nil {
				log.Printf("[WARN] Failed to parse attrs JSON for log: %v", err)
				// If JSON parsing fails, create empty map to avoid breaking the query
				l.Attrs = make(map[string]string)
			}
		} else {
			l.Attrs = make(map[string]string)
		}
		
		logs = append(logs, l)
	}
	
	if err := rows.Err(); err != nil {
		log.Printf("[ERROR] Error iterating rows: %v", err)
		return nil, err
	}
	
	log.Printf("[TRACE] Successfully retrieved %d logs from ClickHouse", len(logs))
	return logs, nil
}

// InsertLogs writes a batch with a prepared statement.
// For higher throughput later, switch to clickhouse native batches (PrepareBatch) under a new method.
func (d *DB) InsertLogs(ctx context.Context, logs []Log) (int64, error) {
	if len(logs) == 0 {
		return 0, nil
	}
	stmt, err := d.PrepareContext(ctx, `
INSERT INTO logs (ts, service, level, msg, attrs, trace_id, span_id)
VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	var n int64
	for _, l := range logs {
		var attrs string
		if l.Attrs != nil {
			b, _ := json.Marshal(l.Attrs)
			attrs = string(b)
		}
		if _, err := stmt.ExecContext(ctx,
			l.Ts, l.Service, l.Level, l.Msg, attrs, l.TraceID, l.SpanID); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
