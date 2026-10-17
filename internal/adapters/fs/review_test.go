package fs

import (
	"testing"
	"time"
)

func TestReview_DebouncerStopWaitsForActiveScan(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	d := newDebouncer(time.Millisecond, func(string) {
		close(started)
		<-release
		close(finished)
	})
	d.schedule("album")
	select {
	case <-started:
	case <-time.After(time.Second):
		d.stop()
		close(release)
		t.Fatal("scan did not start")
	}
	stopped := make(chan struct{})
	go func() {
		d.stop()
		close(stopped)
	}()
	select {
	case <-stopped:
		t.Error("stop returned while its scan callback was still active; caller can now close the database underneath it")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("scan did not finish")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop did not finish after scan completed")
	}
}
