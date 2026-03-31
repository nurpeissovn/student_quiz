package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const frontendFileName = "quiz.html"

func frontendCandidates() []string {
	candidates := []string{}
	if explicit := os.Getenv("FRONTEND_HTML_PATH"); explicit != "" {
		candidates = append(candidates, explicit)
	}
	candidates = append(candidates, frontendFileName)
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), frontendFileName))
	}
	return candidates
}

func loadFrontendHTML() ([]byte, error) {
	for _, candidate := range frontendCandidates() {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("frontend file %q not found", frontendFileName)
}

func serveFrontend(w http.ResponseWriter, _ *http.Request) {
	html, err := loadFrontendHTML()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(html)
}
