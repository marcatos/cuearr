package app_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marcatos/cuearr/internal/app"
	"github.com/marcatos/cuearr/internal/domain"
)

type simultaneousImportStore struct {
	*fakeJobStore
	reads    atomic.Int32
	bothRead chan struct{}
}

func (s *simultaneousImportStore) Get(ctx context.Context, id string) (domain.Job, error) {
	job, err := s.fakeJobStore.Get(ctx, id)
	if n := s.reads.Add(1); n <= 2 {
		if n == 2 {
			close(s.bothRead)
		}
		select {
		case <-s.bothRead:
		case <-ctx.Done():
			return domain.Job{}, ctx.Err()
		}
	}
	return job, err
}

type countingImportClient struct{ requests atomic.Int32 }

func (c *countingImportClient) Ping(context.Context) error { return nil }
func (c *countingImportClient) RequestImport(context.Context, string) error {
	c.requests.Add(1)
	return nil
}

func TestReview_ConcurrentImportRequestsAreClaimedOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	job := completedImportJob("concurrent-import", domain.ImportNone)
	store := &simultaneousImportStore{fakeJobStore: newImportStore(t, job), bothRead: make(chan struct{})}
	client := &countingImportClient{}
	service := app.ImportService{Store: store, Client: client}
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := service.RequestImportForJob(ctx, job.ID, enabledImportSettings())
			done <- err
		}()
	}
	for i := 0; i < 2; i++ {
		<-done
	}
	if got := client.requests.Load(); got != 1 {
		t.Fatalf("concurrent import sent %d external commands for the same job, want 1", got)
	}
}
