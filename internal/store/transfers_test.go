package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

// newTestManager returns a manager whose transfers are driven by fn
// instead of the network.
func newTestManager(concurrency int, fn func(context.Context, *Session, Transfer, func(Progress)) error) *Manager {
	m := NewManager(concurrency, nil)
	m.perform = fn
	return m
}

// waitFor polls until cond holds, or the test times out.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestTransfersRunAndReportProgress(t *testing.T) {
	var changes int
	var mu sync.Mutex
	m := NewManager(2, func() {
		mu.Lock()
		changes++
		mu.Unlock()
	})
	m.perform = func(ctx context.Context, _ *Session, tr Transfer, progress func(Progress)) error {
		progress(Progress{Done: 50, Total: 100})
		progress(Progress{Done: 100, Total: 100})
		return nil
	}

	tr := m.EnqueueDownload(&Session{}, "b", "folder/file.bin", "/tmp/file.bin")
	waitFor(t, "the transfer to finish", func() bool {
		for _, got := range m.List() {
			if got.ID == tr.ID {
				return got.State == StateDone
			}
		}
		return false
	})

	got := m.List()[0]
	if got.Done != 100 || got.Size != 100 {
		t.Errorf("transfer = %d/%d bytes, want 100/100", got.Done, got.Size)
	}
	if got.Name != "file.bin" {
		t.Errorf("Name = %q, want %q", got.Name, "file.bin")
	}
	if got.StartedAt.IsZero() || got.FinishedAt.IsZero() {
		t.Error("the transfer has no start or finish time")
	}
	mu.Lock()
	defer mu.Unlock()
	if changes == 0 {
		t.Error("the change callback was never called")
	}
}

func TestFailedTransferRecordsItsError(t *testing.T) {
	m := newTestManager(1, func(context.Context, *Session, Transfer, func(Progress)) error {
		return errors.New("the bucket is on fire")
	})

	tr := m.EnqueueUpload(&Session{}, "b", "k", "/tmp/k", 10)
	waitFor(t, "the transfer to fail", func() bool {
		got := m.List()[0]
		return got.ID == tr.ID && got.State == StateFailed
	})

	if got := m.List()[0]; got.Error != "the bucket is on fire" {
		t.Errorf("Error = %q, want the failure's message", got.Error)
	}
	if m.Active() != 0 {
		t.Errorf("Active = %d after a failure, want 0", m.Active())
	}
}

func TestCanceledTransferIsMarkedCanceled(t *testing.T) {
	started := make(chan struct{})
	m := newTestManager(1, func(ctx context.Context, _ *Session, _ Transfer, _ func(Progress)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	tr := m.EnqueueDownload(&Session{}, "b", "big.bin", "/tmp/big.bin")
	<-started
	m.Cancel(tr.ID)

	waitFor(t, "the transfer to be canceled", func() bool {
		return m.List()[0].State == StateCanceled
	})
	if m.Active() != 0 {
		t.Errorf("Active = %d after canceling, want 0", m.Active())
	}
}

func TestCancelBeforeAQueuedTransferStarts(t *testing.T) {
	release := make(chan struct{})
	var mu sync.Mutex
	ran := []string{}
	m := newTestManager(1, func(_ context.Context, _ *Session, tr Transfer, _ func(Progress)) error {
		mu.Lock()
		ran = append(ran, tr.Key)
		mu.Unlock()
		<-release
		return nil
	})

	first := m.EnqueueDownload(&Session{}, "b", "first.bin", "/tmp/first.bin")
	second := m.EnqueueDownload(&Session{}, "b", "second.bin", "/tmp/second.bin")
	waitFor(t, "the first transfer to start", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(ran) == 1
	})

	// The second is queued; canceling it must stop it from running.
	m.Cancel(second.ID)
	close(release)

	waitFor(t, "both transfers to settle", func() bool {
		var done int
		for _, got := range m.List() {
			if got.Finished() {
				done++
			}
		}
		return done == 2
	})

	mu.Lock()
	defer mu.Unlock()
	for _, key := range ran {
		if key == "second.bin" {
			t.Error("a canceled queued transfer still ran")
		}
	}
	var firstState, secondState TransferState
	for _, got := range m.List() {
		switch got.ID {
		case first.ID:
			firstState = got.State
		case second.ID:
			secondState = got.State
		}
	}
	if firstState != StateDone {
		t.Errorf("first transfer state = %q, want done", firstState)
	}
	if secondState != StateCanceled {
		t.Errorf("second transfer state = %q, want canceled", secondState)
	}
}

func TestManagerRunsAtMostConcurrencyTransfers(t *testing.T) {
	const concurrency = 2
	const total = 6

	var mu sync.Mutex
	running, peak := 0, 0
	release := make(chan struct{})
	m := newTestManager(concurrency, func(ctx context.Context, _ *Session, _ Transfer, _ func(Progress)) error {
		mu.Lock()
		running++
		if running > peak {
			peak = running
		}
		mu.Unlock()

		select {
		case <-release:
		case <-ctx.Done():
		}

		mu.Lock()
		running--
		mu.Unlock()
		return nil
	})

	for i := range total {
		m.EnqueueDownload(&Session{}, "b", string(rune('a'+i))+".bin", "/tmp/x")
	}
	waitFor(t, "the first batch to fill the slots", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return running == concurrency
	})

	mu.Lock()
	got := peak
	mu.Unlock()
	if got > concurrency {
		t.Errorf("peak concurrency = %d, want at most %d", got, concurrency)
	}

	close(release)
	waitFor(t, "every transfer to finish", func() bool { return m.Active() == 0 })
}

func TestClearFinishedKeepsActiveTransfers(t *testing.T) {
	release := make(chan struct{})
	m := newTestManager(2, func(ctx context.Context, _ *Session, tr Transfer, _ func(Progress)) error {
		if tr.Key == "slow.bin" {
			select {
			case <-release:
			case <-ctx.Done():
			}
		}
		return nil
	})

	slow := m.EnqueueDownload(&Session{}, "b", "slow.bin", "/tmp/slow.bin")
	quick := m.EnqueueDownload(&Session{}, "b", "quick.bin", "/tmp/quick.bin")
	waitFor(t, "the quick transfer to finish", func() bool {
		for _, got := range m.List() {
			if got.ID == quick.ID {
				return got.State == StateDone
			}
		}
		return false
	})

	m.ClearFinished()
	list := m.List()
	if len(list) != 1 || list[0].ID != slow.ID {
		t.Fatalf("List after ClearFinished = %+v, want only the running transfer", list)
	}

	close(release)
	waitFor(t, "the slow transfer to finish", func() bool { return m.Active() == 0 })
}

func TestCloseStopsRunningTransfers(t *testing.T) {
	started := make(chan struct{})
	m := newTestManager(1, func(ctx context.Context, _ *Session, _ Transfer, _ func(Progress)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	m.EnqueueDownload(&Session{}, "b", "k", "/tmp/k")
	<-started
	m.Close()

	waitFor(t, "the transfer to stop", func() bool { return m.Active() == 0 })

	// Enqueueing after Close must not panic or start work.
	m.EnqueueDownload(&Session{}, "b", "later", "/tmp/later")
}

func TestDownloadNameAndUniqueLocalPath(t *testing.T) {
	if got := DownloadName("a/b/c.txt"); got != "c.txt" {
		t.Errorf("DownloadName = %q, want %q", got, "c.txt")
	}
	if got := DownloadName(""); got != "download" {
		t.Errorf("DownloadName of an empty key = %q, want %q", got, "download")
	}

	dir := t.TempDir()
	first := UniqueLocalPath(dir, "photo.jpg")
	if first != dir+"/photo.jpg" {
		t.Errorf("UniqueLocalPath = %q, want %q", first, dir+"/photo.jpg")
	}
	writeFile(t, first)
	second := UniqueLocalPath(dir, "photo.jpg")
	if second != dir+"/photo 2.jpg" {
		t.Errorf("UniqueLocalPath = %q, want %q", second, dir+"/photo 2.jpg")
	}
	writeFile(t, second)
	third := UniqueLocalPath(dir, "photo.jpg")
	if third != dir+"/photo 3.jpg" {
		t.Errorf("UniqueLocalPath = %q, want %q", third, dir+"/photo 3.jpg")
	}
}

func TestFolderName(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"backups/":         "backups",
		"a/b/c/":           "c",
		"/leading/":        "leading",
		"2025/db-07.sql/":  "db-07.sql",
		"trailing/slashes": "slashes",
	}
	for in, want := range cases {
		if got := FolderName(in); got != want {
			t.Errorf("FolderName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestContentTypeFor(t *testing.T) {
	cases := map[string]string{
		"a/b/photo.PNG":  "image/png",
		"clip.mp4":       "video/mp4",
		"notes.md":       "text/plain; charset=utf-8",
		"unknown.xyzzy":  "application/octet-stream",
		"no-extension":   "application/octet-stream",
		"data.json":      "application/json",
		"archive.tar.gz": "application/gzip",
	}
	for in, want := range cases {
		if got := contentTypeFor(in); got != want {
			t.Errorf("contentTypeFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMultipartPartSizeGrowsWithTheFile(t *testing.T) {
	small := multipartPartSize(1 << 20)
	medium := multipartPartSize(1 << 30)
	huge := multipartPartSize(64 << 30)
	if small > medium {
		t.Errorf("part size shrank with size: %d then %d", small, medium)
	}
	if medium > huge {
		t.Errorf("part size shrank with size: %d then %d", medium, huge)
	}
	if small < 5<<20 {
		t.Errorf("part size %d is below S3's 5 MiB minimum", small)
	}
}

func TestSortEntriesPutsFoldersFirst(t *testing.T) {
	items := []Entry{
		{Name: "zebra.txt"},
		{Name: "Banana", Folder: true},
		{Name: "apple", Folder: true},
		{Name: "Apple.txt"},
	}
	sortEntries(items)
	want := []string{"apple", "Banana", "Apple.txt", "zebra.txt"}
	for i, name := range want {
		if items[i].Name != name {
			t.Fatalf("position %d = %q, want %q (order: %v)", i, items[i].Name, name, names(items))
		}
	}
}

func names(items []Entry) []string {
	out := make([]string, len(items))
	for i, e := range items {
		out[i] = e.Name
	}
	return out
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTransfersStartInQueueOrder checks the dispatcher's queue is FIFO: a
// transfer never starts before an older one. One slot is used so the
// start order is fully determined, rather than left to the scheduler.
func TestTransfersStartInQueueOrder(t *testing.T) {
	const n = 8
	var mu sync.Mutex
	order := []string{}
	m := newTestManager(1, func(_ context.Context, _ *Session, tr Transfer, _ func(Progress)) error {
		mu.Lock()
		order = append(order, tr.Key)
		mu.Unlock()
		return nil
	})

	want := make([]string, n)
	for i := range n {
		key := string(rune('a'+i)) + ".bin"
		want[i] = key
		m.EnqueueDownload(&Session{}, "b", key, "/tmp/"+key)
	}
	waitFor(t, "every transfer to finish", func() bool { return m.Active() == 0 })

	mu.Lock()
	defer mu.Unlock()
	if len(order) != n {
		t.Fatalf("ran %d transfers, want %d", len(order), n)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("transfer %d ran %q, want %q (order: %v)", i, order[i], want[i], order)
		}
	}
}
