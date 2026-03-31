package handlers

import (
	"net/http"

	"github.com/finset/app/internal/models"
	"github.com/google/uuid"
)

func (h *Handler) SubmitQuizAttempt(w http.ResponseWriter, r *http.Request) {
	var req models.SubmitQuizAttemptRequest
	if err := parseBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Normalize()
	if errMsg := req.Validate(); errMsg != "" {
		writeError(w, http.StatusBadRequest, errMsg)
		return
	}

	attemptID := uuid.New().String()
	studentID := uuid.New().String()
	pool := h.getDB()
	if pool != nil {
		saved, err := pool.SaveQuizAttempt(r.Context(), attemptID, studentID, req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save quiz attempt: "+err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, saved)
		return
	}
	writeJSON(w, http.StatusCreated, h.saveQuizAttemptMemory(attemptID, studentID, req))
}

func (h *Handler) GetQuizDashboard(w http.ResponseWriter, r *http.Request) {
	pool := h.getDB()
	if pool == nil {
		writeJSON(w, http.StatusOK, h.getQuizDashboardMemory())
		return
	}
	dashboard, err := pool.GetQuizDashboard(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load dashboard: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dashboard)
}
