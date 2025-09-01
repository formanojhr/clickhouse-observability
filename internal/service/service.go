package service

import (
	"context"
	"time"

	"go-log-service/internal/batcher"
	"go-log-service/internal/db"
	pb "go-log-service/internal/service/pb/proto"
)

type LogServer struct {
	b *batcher.Batcher
	pb.UnimplementedLogServiceServer
}

func NewLogServer(b *batcher.Batcher) *LogServer {
	return &LogServer{b: b}
}

func (s *LogServer) BatchWrite(ctx context.Context, req *pb.BatchWriteRequest) (*pb.BatchWriteResponse, error) {
	// Convert and enqueue; return the count immediately (fire-and-forget semantics).
	out := make([]db.Log, 0, len(req.Entries))
	now := time.Now().UTC()

	for _, e := range req.Entries {
		ts, err := time.Parse(time.RFC3339Nano, e.Ts)
		if err != nil {
			if t2, err2 := time.Parse(time.RFC3339, e.Ts); err2 == nil {
				ts = t2
			} else {
				ts = now
			}
		}
		out = append(out, db.Log{
			Ts:      ts,
			Service: e.Service,
			Level:   e.Level,
			Msg:     e.Msg,
			Attrs:   e.Attrs,
			TraceID: e.TraceId,
			SpanID:  e.SpanId,
		})
	}
	s.b.SubmitMany(out)
	return &pb.BatchWriteResponse{Written: uint64(len(out))}, nil
}
