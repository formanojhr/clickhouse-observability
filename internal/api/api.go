package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
	
	"go-log-service/internal/db"
)

type API struct{
	db *db.DB
}

func New(db *db.DB) *API {
	return &API{db: db}
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		log.Println("[TRACE] /api/ping called")
		w.Write([]byte("pong"))
	})
	
	mux.HandleFunc("/v1/logs", a.handleQueryLogs)
}

func (a *API) handleQueryLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("[ERROR] Invalid method %s for /v1/logs", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	log.Printf("[TRACE] Query logs request received: %s", r.URL.String())
	
	// Parse query parameters
	service := r.URL.Query().Get("service")
	if service == "" {
		log.Printf("[ERROR] Missing required parameter: service")
		http.Error(w, "Missing required parameter: service", http.StatusBadRequest)
		return
	}
	
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		log.Printf("[ERROR] Invalid from parameter: %v", err)
		http.Error(w, "Invalid from parameter. Use RFC3339 format (e.g., 2023-01-01T00:00:00Z)", http.StatusBadRequest)
		return
	}
	
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		log.Printf("[ERROR] Invalid to parameter: %v", err)
		http.Error(w, "Invalid to parameter. Use RFC3339 format (e.g., 2023-01-01T00:00:00Z)", http.StatusBadRequest)
		return
	}
	
	level := r.URL.Query().Get("level")
	user := r.URL.Query().Get("user")
	
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		} else {
			log.Printf("[ERROR] Invalid limit parameter: %s", limitStr)
			http.Error(w, "Invalid limit parameter. Must be a positive integer", http.StatusBadRequest)
			return
		}
	}
	
	// Validate time range
	if from.After(to) {
		log.Printf("[ERROR] Invalid time range: from (%v) is after to (%v)", from, to)
		http.Error(w, "Invalid time range: 'from' must be before 'to'", http.StatusBadRequest)
		return
	}
	
	log.Printf("[TRACE] Querying logs: service=%s, level=%s, user=%s, from=%v, to=%v, limit=%d", 
		service, level, user, from, to, limit)
	
	// Query the database
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	
	logs, err := a.db.QueryLogs(ctx, service, level, user, from, to, limit)
	if err != nil {
		log.Printf("[ERROR] Database query failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	log.Printf("[TRACE] Query completed successfully. Found %d logs", len(logs))
	
	// Prepare response
	response := map[string]interface{}{
		"logs": logs,
		"count": len(logs),
		"query": map[string]interface{}{
			"service": service,
			"level":   level,
			"user":    user,
			"from":    from.Format(time.RFC3339),
			"to":      to.Format(time.RFC3339),
			"limit":   limit,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[ERROR] Failed to encode response: %v", err)
		// Response already started, can't change status code
	}
}



