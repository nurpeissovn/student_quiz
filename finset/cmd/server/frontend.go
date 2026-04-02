package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const frontendFileName = "quiz.html"
const sqlQuizFileName = "sql_quiz.html"

func frontendCandidates(fileName string) []string {
	candidates := []string{}
	if fileName == frontendFileName {
		if explicit := os.Getenv("FRONTEND_HTML_PATH"); explicit != "" {
			candidates = append(candidates, explicit)
		}
	}
	candidates = append(candidates, fileName)
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), fileName))
	}
	return candidates
}

func loadFrontendHTML(fileName string) ([]byte, error) {
	for _, candidate := range frontendCandidates(fileName) {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("frontend file %q not found", fileName)
}

func serveFrontendFile(w http.ResponseWriter, fileName string) {
	html, err := loadFrontendHTML(fileName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(html)
}

func serveFrontend(w http.ResponseWriter, _ *http.Request) {
	serveFrontendFile(w, frontendFileName)
}

func serveSQLQuizFrontend(w http.ResponseWriter, _ *http.Request) {
	serveFrontendFile(w, sqlQuizFileName)
}
