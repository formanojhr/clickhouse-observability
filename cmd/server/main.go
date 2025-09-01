package main

import (
	"context"
	"go-log-service/internal/api"
	"go-log-service/internal/batcher"
	"go-log-service/internal/db"
	"go-log-service/internal/service"
	pb "go-log-service/internal/service/pb/proto"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("[TRACE] Starting go-log-service main...")
	httpAddr := getenv("HTTP_ADDR", ":8080")
	grpcAddr := getenv("GRPC_ADDR", ":8081")
	dsn := getenv("DATABASE_URL", "clickhouse://demo:@localhost:9000/observability?dial_timeout=10s&read_timeout=5s&compression=lz4")
	batchSize := getenvInt("INGEST_BATCH_SIZE", 500)
	flushMs := getenvInt("INGEST_MAX_DELAY_MS", 100)

	log.Printf("[TRACE] Config: HTTP_ADDR=%s, GRPC_ADDR=%s, DATABASE_URL=%s, BATCH_SIZE=%d, FLUSH_MS=%d", httpAddr, grpcAddr, dsn, batchSize, flushMs)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// DB
	log.Println("[TRACE] Opening database connection...")
	store, err := db.Open(ctx, dsn)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	log.Println("[TRACE] Database connection established.")

	// Batcher
	log.Println("[TRACE] Initializing batcher...")
	b := batcher.New(store, batchSize, time.Duration(flushMs)*time.Millisecond)
	go func() {
		log.Println("[TRACE] Batcher started.")
		b.Run(ctx)
		log.Println("[TRACE] Batcher stopped.")
	}()

	// HTTP API
	api := api.New(store)
	mux := http.NewServeMux()
	
	// Health endpoints
	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	
	// Register API routes
	api.RegisterRoutes(mux)
	
	httpSrv := &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		log.Printf("[TRACE] HTTP server listening on %s", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
		log.Println("[TRACE] HTTP server stopped.")
	}()

	// gRPC
	log.Println("[TRACE] Starting gRPC server...")
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	gs := grpc.NewServer()
	pb.RegisterLogServiceServer(gs, service.NewLogServer(b))
	reflection.Register(gs)
	go func() {
		log.Printf("[TRACE] gRPC server listening on %s", grpcAddr)
		if err := gs.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
		log.Println("[TRACE] gRPC server stopped.")
	}()

	log.Println("[TRACE] Server is running. Waiting for shutdown signal...")
	<-ctx.Done()
	log.Printf("[TRACE] Shutdown signal received. Shutting down...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)
	gs.GracefulStop()
	log.Println("[TRACE] Server shutdown complete.")
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func getenvInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}
