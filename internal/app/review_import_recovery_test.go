package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestReview_PollerRecoversInterruptedImport(t *testing.T) {
	now := time.Now().UTC()
	job := completedImportJob("interrupted-request", domain.ImportRequested)
	job.ImportRequestedAt = now.Add(-2 * domain.ImportRequestCooldown)
	recent := completedImportJob("active-request", domain.ImportRequested)
	recent.ImportRequestedAt = now.Add(-time.Second)
	store := newImportStore(t, job, recent)
	client := &fakeLidarrClient{}
	service := app.ImportService{Store: store, Client: client, Clock: func() time.Time { return now }}
	settings := enabledImportSettings()
	settings.LidarrPollIntervalSec = 30
	if _, err := service.PollPendingImports(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	if len(client.paths) != 1 {
		t.Fatalf("stale requested import was not recovered: requests=%d, want 1", len(client.paths))
	}
}
