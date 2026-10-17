package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marcatos/cuearr/internal/domain"
)

func TestCreateJobRejectsTrailingAndOversizedJSON(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
	}{
		{"second value", `{"path":"/album"} {"path":"/other"}`, http.StatusBadRequest},
		{"trailing garbage", `{"path":"/album"} garbage`, http.StatusBadRequest},
		{"oversized whitespace", `{"path":"/album"}` + strings.Repeat(" ", maxRequestBody), http.StatusRequestEntityTooLarge},
		{"valid whitespace", "{\"path\":\"/album\"}\n ", http.StatusCreated},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			srv := New(Deps{CreateJob: func(context.Context, string) (domain.Job, bool, error) {
				called = true
				return domain.Job{ID: "job"}, true, nil
			}})
			rec := httptest.NewRecorder()
			srv.handleCreateJob(rec, httptest.NewRequest(http.MethodPost, "/api/v1/jobs", strings.NewReader(tt.body)))
			if rec.Code != tt.status {
				t.Fatalf("status=%d want=%d", rec.Code, tt.status)
			}
			if called != (tt.status == http.StatusCreated) {
				t.Fatalf("unexpected create call=%t", called)
			}
		})
	}
}

func TestLidarrHookRejectsOversizedTestBody(t *testing.T) {
	srv := New(Deps{})
	rec := httptest.NewRecorder()
	body := `{"eventType":"Test"}` + strings.Repeat(" ", maxRequestBody)
	srv.handleLidarrHook(rec, httptest.NewRequest(http.MethodPost, "/api/v1/hooks/lidarr", strings.NewReader(body)))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", rec.Code)
	}
}
