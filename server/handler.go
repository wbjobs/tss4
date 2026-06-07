package server

import (
	"encoding/json"
	"fmt"
	"gorilla-tsdb/block"
	"gorilla-tsdb/tsdb"
	"net/http"
	"strconv"
)

type Handler struct {
	db *tsdb.TSDB
}

func NewHandler(db *tsdb.TSDB) *Handler {
	return &Handler{db: db}
}

type WriteRequest struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type WriteBatchRequest struct {
	Points []WriteRequest `json:"points"`
}

type WriteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type QueryResponse struct {
	Success bool              `json:"success"`
	Points  []block.DataPoint `json:"points,omitempty"`
	Message string            `json:"message,omitempty"`
}

type StatsResponse struct {
	Success        bool   `json:"success"`
	BlockCount     int    `json:"block_count"`
	ActivePoints   int    `json:"active_points"`
	Message        string `json:"message,omitempty"`
}

func (h *Handler) Write(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: "method not allowed",
		})
		return
	}

	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: fmt.Sprintf("invalid request body: %v", err),
		})
		return
	}

	if req.Timestamp <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: "invalid timestamp",
		})
		return
	}

	point := block.DataPoint{
		Timestamp: req.Timestamp,
		Value:     req.Value,
	}

	if err := h.db.Write(point); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: fmt.Sprintf("write failed: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(WriteResponse{
		Success: true,
	})
}

func (h *Handler) WriteBatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: "method not allowed",
		})
		return
	}

	var req WriteBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: fmt.Sprintf("invalid request body: %v", err),
		})
		return
	}

	if len(req.Points) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: "empty points",
		})
		return
	}

	points := make([]block.DataPoint, 0, len(req.Points))
	for i, p := range req.Points {
		if p.Timestamp <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(WriteResponse{
				Success: false,
				Message: fmt.Sprintf("invalid timestamp at index %d", i),
			})
			return
		}
		points = append(points, block.DataPoint{
			Timestamp: p.Timestamp,
			Value:     p.Value,
		})
	}

	if err := h.db.WriteBatch(points); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: fmt.Sprintf("write failed: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(WriteResponse{
		Success: true,
		Message: fmt.Sprintf("wrote %d points", len(points)),
	})
}

func (h *Handler) Query(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(QueryResponse{
			Success: false,
			Message: "method not allowed",
		})
		return
	}

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	downsampleStr := r.URL.Query().Get("downsample")

	if startStr == "" || endStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{
			Success: false,
			Message: "start and end parameters are required",
		})
		return
	}

	startTime, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{
			Success: false,
			Message: "invalid start timestamp",
		})
		return
	}

	endTime, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{
			Success: false,
			Message: "invalid end timestamp",
		})
		return
	}

	if startTime > endTime {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{
			Success: false,
			Message: "start must be <= end",
		})
		return
	}

	downsample := 0
	if downsampleStr != "" {
		downsample, err = strconv.Atoi(downsampleStr)
		if err != nil || downsample < 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(QueryResponse{
				Success: false,
				Message: "invalid downsample parameter",
			})
			return
		}
	}

	points, err := h.db.QueryWithDownsample(startTime, endTime, downsample)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{
			Success: false,
			Message: fmt.Sprintf("query failed: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(QueryResponse{
		Success: true,
		Points:  points,
	})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(StatsResponse{
			Success: false,
			Message: "method not allowed",
		})
		return
	}

	entries := h.db.Index().Entries()
	activeBlock := h.db.ActiveBlock()
	activePoints := 0
	if activeBlock != nil {
		activePoints = len(activeBlock.Points)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StatsResponse{
		Success:      true,
		BlockCount:   len(entries),
		ActivePoints: activePoints,
	})
}

func (h *Handler) Flush(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: "method not allowed",
		})
		return
	}

	if err := h.db.Flush(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(WriteResponse{
			Success: false,
			Message: fmt.Sprintf("flush failed: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(WriteResponse{
		Success: true,
		Message: "flushed successfully",
	})
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/write", h.Write)
	mux.HandleFunc("/write/batch", h.WriteBatch)
	mux.HandleFunc("/query", h.Query)
	mux.HandleFunc("/stats", h.Stats)
	mux.HandleFunc("/flush", h.Flush)
}
