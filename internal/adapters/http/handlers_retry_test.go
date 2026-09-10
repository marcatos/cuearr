package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/domain"
)

func TestRetryJob_RequeuesFailed(t *testing.T) {
	at := time.Date(2026, 9, 10, 16, 0, 0, 0, time.UTC)
	jobs := &memJobStore{
		jobs: map[string]domain.Job{
			"job-failed": {
				ID:           "job-failed",
				Status:       domain.JobFailed,
				Error:        "permission denied",
				Log:          "tool output",
				AttemptCount: 2,
				AttemptLog: []domain.JobAttempt{
					{Number: 1, At: at, Error: "first"},
					{Number: 2, At: at, Error: "permission denied"},
				},
				StartedAt:  at,
				FinishedAt: at,
				CreatedAt:  at,
			},
		},
	}
	ts := newTestServer(t, jobs, nil)
	defer ts.Close()

	res, err := apiPost(ts.URL+"/api/v1/jobs/job-failed/retry", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}
	var body struct {
		ID           string `json:"id"`
		Status       string `json:"status"`
		Error        string `json:"error"`
		Log          string `json:"log"`
		AttemptCount int    `json:"attempt_count"`
		Attempts     []struct {
			Number int    `json:"n"`
			Error  string `json:"error"`
		} `json:"attempts"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != domain.JobQueued || body.Error != "" || body.Log != "" {
		t.Fatalf("body=%+v", body)
	}
	if body.AttemptCount != 0 || len(body.Attempts) != 2 || body.Attempts[1].Error != "permission denied" {
		t.Fatalf("attempts=%+v", body.Attempts)
	}
	stored := jobs.jobs["job-failed"]
	if stored.Status != domain.JobQueued || stored.AttemptCount != 0 {
		t.Fatalf("stored=%+v", stored)
	}
}

func TestRetryJob_ConflictWhenNotFailed(t *testing.T) {
	jobs := &memJobStore{
		jobs: map[string]domain.Job{
			"job-queued": {ID: "job-queued", Status: domain.JobQueued, CreatedAt: time.Now().UTC()},
		},
	}
	ts := newTestServer(t, jobs, nil)
	defer ts.Close()

	res, err := apiPost(ts.URL+"/api/v1/jobs/job-queued/retry", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d", res.StatusCode)
	}
}

func TestRetryJob_NotFound(t *testing.T) {
	ts := newTestServer(t, &memJobStore{}, nil)
	defer ts.Close()

	res, err := apiPost(ts.URL+"/api/v1/jobs/missing/retry", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d", res.StatusCode)
	}
}
