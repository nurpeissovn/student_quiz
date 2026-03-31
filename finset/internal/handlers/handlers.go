package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/finset/app/internal/db"
	"github.com/finset/app/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	mu sync.RWMutex
	DB *db.Pool
}

func New(d *db.Pool) *Handler {
	return &Handler{DB: d}
}

func (h *Handler) SetDB(pool *db.Pool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.DB != nil && h.DB != pool {
		h.DB.Close()
	}
	h.DB = pool
}

func (h *Handler) getDB() *db.Pool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.DB
}

func (h *Handler) requireDB(w http.ResponseWriter) *db.Pool {
	pool := h.getDB()
	if pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database is still starting")
		return nil
	}
	return pool
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	log.Printf("API error %d: %s", status, msg)
	writeJSON(w, status, map[string]string{"error": msg})
}

func parseBody(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	txs, err := pool.ListTransactions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch transactions: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, txs)
}

func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	id := chi.URLParam(r, "id")
	tx, err := pool.GetTransaction(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "transaction not found: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tx)
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	var req models.CreateTransactionRequest
	if err := parseBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.Type = strings.TrimSpace(req.Type)
	req.Category = strings.TrimSpace(req.Category)
	req.Method = strings.TrimSpace(req.Method)
	req.Note = strings.TrimSpace(req.Note)
	req.Date = strings.TrimSpace(req.Date)

	if req.Method == "" {
		req.Method = "Cash"
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}

	if errMsg := req.Validate(); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	id := uuid.New().String()
	tx, err := pool.CreateTransaction(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create transaction: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tx)
}

func (h *Handler) DeleteAllTransactions(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	n, err := pool.DeleteAllTransactions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete all: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"deleted": n})
}

func (h *Handler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	id := chi.URLParam(r, "id")
	found, err := pool.DeleteTransaction(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete transaction: "+err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	stats, err := pool.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch stats: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) GetMonthlyFlow(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	months := 7
	if m := r.URL.Query().Get("months"); m != "" {
		var n int
		if json.Unmarshal([]byte(m), &n) == nil && n > 0 && n <= 24 {
			months = n
		}
	}
	rows, err := pool.GetMonthlyFlow(r.Context(), months)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch monthly flow: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *Handler) GetCategoryBreakdown(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	rows, err := pool.GetCategoryBreakdown(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch category breakdown: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *Handler) ImportTransactions(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	type importBody struct {
		Transactions []models.Transaction `json:"transactions"`
	}
	var body importBody
	if err := parseBody(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: expected {transactions:[…]}")
		return
	}
	if len(body.Transactions) == 0 {
		writeError(w, http.StatusBadRequest, "no transactions provided")
		return
	}
	for i := range body.Transactions {
		if body.Transactions[i].ID == "" {
			body.Transactions[i].ID = uuid.New().String()
		}
		if body.Transactions[i].CreatedAt.IsZero() {
			body.Transactions[i].CreatedAt = time.Now()
		}
	}
	inserted, err := pool.BulkInsert(r.Context(), body.Transactions)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"inserted": inserted,
		"skipped":  len(body.Transactions) - inserted,
		"total":    len(body.Transactions),
	})
}

func (h *Handler) Debug(w http.ResponseWriter, r *http.Request) {
	pool := h.requireDB(w)
	if pool == nil {
		return
	}
	ctx := r.Context()
	result := map[string]any{}

	// Test basic query
	var count int
	err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&count)
	if err != nil {
		result["count_error"] = err.Error()
	} else {
		result["count"] = count
	}

	// Test monthly flow query directly
	cutoff := "2020-01-01"
	rows, err := pool.Query(ctx, `
		SELECT
			TO_CHAR(DATE_TRUNC('month', date), 'Mon') AS month,
			TO_CHAR(DATE_TRUNC('month', date), 'YYYY') AS year,
			COALESCE(SUM(CASE WHEN type='income'  THEN amount ELSE 0 END), 0)::float8 AS income,
			COALESCE(SUM(CASE WHEN type='expense' THEN amount ELSE 0 END), 0)::float8 AS expense
		FROM transactions
		WHERE date >= $1::DATE
		GROUP BY DATE_TRUNC('month', date)
		ORDER BY DATE_TRUNC('month', date) ASC
	`, cutoff)
	if err != nil {
		result["flow_query_error"] = err.Error()
	} else {
		defer rows.Close()
		var flowRows []map[string]any
		for rows.Next() {
			var month, year string
			var income, expense float64
			if err := rows.Scan(&month, &year, &income, &expense); err != nil {
				result["flow_scan_error"] = err.Error()
				break
			}
			flowRows = append(flowRows, map[string]any{"month": month, "year": year, "income": income, "expense": expense})
		}
		result["flow_rows"] = flowRows
		if err := rows.Err(); err != nil {
			result["flow_rows_error"] = err.Error()
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	pool := h.getDB()
	if pool == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "starting", "database_connected": false})
		return
	}
	if err := pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "degraded", "database_connected": false, "database_error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "database_connected": true})
}
