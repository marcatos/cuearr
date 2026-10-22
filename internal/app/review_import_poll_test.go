package app_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/adapters/store/sqlite"
	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestReview_PollerFindsPendingImportsBeyondRecentHistory(t *testing.T) {
	ctx := context.Background()
	// Use the real store: the shared fake ignores List's ordering and limit.
	store, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	old := completedImportJob("old-pending", domain.ImportFailed)
	old.CreatedAt = now.Add(-time.Hour)
	if _, err := store.Create(ctx, old); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		job := completedImportJob(fmt.Sprintf("new-imported-%d", i), domain.ImportImported)
		job.CreatedAt = now.Add(time.Duration(i) * time.Second)
		if _, err := store.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	client := &fakeLidarrClient{}
	settings := enabledImportSettings()
	settings.LidarrPollIntervalSec = 30
	if _, err := (app.ImportService{Store: store, Client: client}).PollPendingImports(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if len(client.paths) != 1 || client.paths[0] != old.OutDir {
		t.Fatalf("old pending import starved behind completed history: requests=%v", client.paths)
	}
}

func TestPollPendingImportsProcessesAllPages(t *testing.T) {
	store := newImportStore(t)
	for i := 0; i < 1001; i++ {
		job := completedImportJob(fmt.Sprintf("pending-%04d", i), domain.ImportNone)
		if _, err := store.Create(context.Background(), job); err != nil {
			t.Fatal(err)
		}
	}
	client := &countingImportClient{}
	settings := enabledImportSettings()
	settings.LidarrPollIntervalSec = 30
	n, err := (app.ImportService{Store: store, Client: client}).PollPendingImports(context.Background(), settings)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1001 || client.requests.Load() != 1001 {
		t.Fatalf("processed=%d requests=%d", n, client.requests.Load())
	}
}
