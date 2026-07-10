package sidecar

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"palpanel/sav-cli/internal/buildinfo"
	"palpanel/sav-cli/internal/indexer"
)

type Server struct {
	mux *http.ServeMux
}

type indexRequest struct {
	SaveDir        string `json:"save_dir"`
	SavePath       string `json:"save_path"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type envelope struct {
	OK       bool          `json:"ok"`
	Data     any           `json:"data,omitempty"`
	Error    *errorPayload `json:"error,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New() *Server {
	s := &Server{mux: http.NewServeMux()}
	s.mux.HandleFunc("/health", s.health)
	s.mux.HandleFunc("/index", s.index)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, envelope{OK: false, Error: &errorPayload{Code: "method_not_allowed", Message: "method not allowed"}})
		return
	}
	info := buildinfo.Current()
	writeJSON(w, http.StatusOK, envelope{OK: true, Data: map[string]any{
		"status":        "ok",
		"version":       indexer.IndexVersion,
		"parser":        indexer.ParserName,
		"build_version": info.Version,
		"commit":        info.Commit,
		"build_time":    info.BuildTime,
	}})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, envelope{OK: false, Error: &errorPayload{Code: "method_not_allowed", Message: "method not allowed"}})
		return
	}
	defer r.Body.Close()
	var req indexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, envelope{OK: false, Error: &errorPayload{Code: "bad_request", Message: err.Error()}})
		return
	}
	savePath := strings.TrimSpace(firstNonEmpty(req.SaveDir, req.SavePath))
	if savePath == "" {
		writeJSON(w, http.StatusBadRequest, envelope{OK: false, Error: &errorPayload{Code: "save_path_required", Message: "save_dir is required"}})
		return
	}
	idx, err := indexer.Build(savePath)
	if err != nil {
		status := http.StatusUnprocessableEntity
		payload := errorPayload{Code: indexer.CodeIndexFailed, Message: err.Error()}
		var indexed *indexer.Error
		if errors.As(err, &indexed) {
			payload.Code = indexed.Code
			payload.Message = indexed.Message
			if indexed.Code == indexer.CodeSavePathNotFound || indexed.Code == indexer.CodeLevelSavNotFound {
				status = http.StatusNotFound
			}
		}
		writeJSON(w, status, envelope{OK: false, Error: &payload, Warnings: idx.Warnings})
		return
	}
	writeJSON(w, http.StatusOK, envelope{OK: true, Data: idx})
}

func ListenAndServe(addr string) error {
	return NewHTTPServer(addr).ListenAndServe()
}

func NewHTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           New(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		status = http.StatusInternalServerError
		body = []byte(`{"ok":false,"error":{"code":"json_encode_failed","message":"failed to encode response"}}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
