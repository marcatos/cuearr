package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/marcatos/cuearr/internal/domain"
)

var errLidarrPathMissing = errors.New("download path not found in payload")

func (s *Server) handleLidarrHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.deps.CreateJob == nil {
		writeError(w, http.StatusInternalServerError, "create job not configured")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rawPath, err := extractLidarrScanPath(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	scanPath, err := lidarrDirForScan(rawPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	job, created, err := s.deps.CreateJob(r.Context(), scanPath)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, domain.ErrImageNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	code := http.StatusCreated
	if !created {
		code = http.StatusOK
	}
	writeJSON(w, code, map[string]any{
		"job_id":  job.ID,
		"created": created,
	})
}

// extractLidarrScanPath reads Lidarr Connect webhooks and custom-script JSON bodies.
func extractLidarrScanPath(body []byte) (string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return "", errors.New("invalid JSON body")
	}
	for _, key := range []string{"path", "DownloadPath", "DownloadFolder"} {
		if p := jsonStringField(top, key); p != "" {
			return p, nil
		}
	}
	for _, nestKey := range []string{"environment", "env"} {
		if p := nestedStringField(top, nestKey, "DownloadPath", "DownloadFolder", "path"); p != "" {
			return p, nil
		}
	}
	return "", errLidarrPathMissing
}

func jsonStringField(obj map[string]json.RawMessage, key string) string {
	raw, ok := obj[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s == "" {
		return ""
	}
	return s
}

func nestedStringField(obj map[string]json.RawMessage, nestKey string, keys ...string) string {
	raw, ok := obj[nestKey]
	if !ok {
		return ""
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(raw, &nested); err != nil {
		return ""
	}
	for _, key := range keys {
		if p := jsonStringField(nested, key); p != "" {
			return p
		}
	}
	return ""
}

func lidarrDirForScan(path string) (string, error) {
	path = filepath.Clean(path)
	if path == "" || path == "." {
		return "", errors.New("path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if ext := filepath.Ext(path); ext != "" {
				return filepath.Dir(path), nil
			}
			return path, nil
		}
		return "", err
	}
	if info.IsDir() {
		return path, nil
	}
	return filepath.Dir(path), nil
}
