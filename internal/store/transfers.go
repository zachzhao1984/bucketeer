package store

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"sync"
	"time"
)

// TransferKind tells an upload from a download.
type TransferKind string

// The kinds of transfer.
const (
	KindUpload   TransferKind = "upload"
	KindDownload TransferKind = "download"
)

// TransferState is where a transfer stands.
type TransferState string

// The states of a transfer.
const (
	StateQueued   TransferState = "queued"
	StateRunning  TransferState = "running"
	StateDone     TransferState = "done"
	StateFailed   TransferState = "failed"
	StateCanceled TransferState = "canceled"
)

// Transfer is one upload or download.
type Transfer struct {
	ID     string       `json:"id"`
	Kind   TransferKind `json:"kind"`
	Bucket string       `json:"bucket"`
	Key    string       `json:"key"`
	// Name is the object's file name, for display.
	Name string `json:"name"`
	// Path is the local file's path.
	Path string `json:"path"`
	// Size is the number of bytes to move, known once the transfer
	// starts; zero while it is queued.
	Size  int64         `json:"size"`
	Done  int64         `json:"done"`
	State TransferState `json:"state"`
	Error string        `json:"error,omitempty"`
	// StartedAt and FinishedAt bracket the transfer; zero when they have
	// not happened yet.
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
}

// Finished reports whether the transfer has stopped, one way or another.
func (t Transfer) Finished() bool {
	switch t.State {
	case StateDone, StateFailed, StateCanceled:
		return true
	}
	return false
}

// Manager runs transfers a few at a time and reports their progress.
// The callback is called from the transferring goroutines, so it must be
// quick and safe to call concurrently.
//
// Transfers start in the order they were queued, so the list the user
// sees is the order the work happens in.
type Manager struct {
	mu    sync.Mutex
	items map[string]*transferItem
	// order holds every transfer id, newest last, for List.
	order []string
	// queue holds the ids still waiting for a slot, oldest first.
	queue    []string
	slots    chan struct{}
	onChange func()
	closed   bool
	closedCh chan struct{}
	// wake tells the dispatcher that the queue is not empty.
	wake chan struct{}
	// perform runs one transfer. Tests replace it; NewManager sets it to
	// the real upload and download.
	perform func(context.Context, *Session, Transfer, func(Progress)) error
}

type transferItem struct {
	Transfer
	cancel context.CancelFunc
	// session keeps the connection the transfer runs on alive.
	session *Session
}

// NewManager returns a manager running at most concurrency transfers at
// once. onChange is called whenever a transfer's state changes.
func NewManager(concurrency int, onChange func()) *Manager {
	if concurrency < 1 {
		concurrency = 1
	}
	m := &Manager{
		items:    map[string]*transferItem{},
		slots:    make(chan struct{}, concurrency),
		onChange: onChange,
		closedCh: make(chan struct{}),
		wake:     make(chan struct{}, 1),
		perform:  performTransfer,
	}
	go m.dispatch()
	return m
}

// notify calls the change callback, unless the manager is closed.
func (m *Manager) notify() {
	m.mu.Lock()
	fn := m.onChange
	closed := m.closed
	m.mu.Unlock()
	if fn != nil && !closed {
		fn()
	}
}

// Enqueue adds a download of bucket/key into dest.
func (m *Manager) EnqueueDownload(sess *Session, bucket, key, dest string) Transfer {
	return m.enqueue(Transfer{
		Kind:   KindDownload,
		Bucket: bucket,
		Key:    key,
		Name:   path.Base(key),
		Path:   dest,
	}, sess)
}

// EnqueueUpload adds an upload of the local file src to bucket/key.
func (m *Manager) EnqueueUpload(sess *Session, bucket, key, src string, size int64) Transfer {
	return m.enqueue(Transfer{
		Kind:   KindUpload,
		Bucket: bucket,
		Key:    key,
		Name:   path.Base(key),
		Path:   src,
		Size:   size,
	}, sess)
}

func (m *Manager) enqueue(t Transfer, sess *Session) Transfer {
	t.ID = newID()
	t.State = StateQueued

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		t.State = StateCanceled
		return t
	}
	m.items[t.ID] = &transferItem{Transfer: t, session: sess}
	m.order = append(m.order, t.ID)
	m.queue = append(m.queue, t.ID)
	m.mu.Unlock()

	m.notify()
	m.signal()
	return t
}

// signal wakes the dispatcher without blocking.
func (m *Manager) signal() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// dispatch starts queued transfers in order, one slot at a time. It is
// the only goroutine that takes a slot, so transfers start in the order
// they were queued.
func (m *Manager) dispatch() {
	for {
		id, ok := m.next()
		if !ok {
			select {
			case <-m.wake:
				continue
			case <-m.closedCh:
				return
			}
		}
		select {
		case m.slots <- struct{}{}:
		case <-m.closedCh:
			return
		}
		go m.run(id)
	}
}

// next takes the oldest queued transfer that has not stopped, dropping the
// ones that were canceled while they waited.
func (m *Manager) next() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for len(m.queue) > 0 {
		id := m.queue[0]
		m.queue = m.queue[1:]
		item, ok := m.items[id]
		if !ok || item.Finished() {
			continue
		}
		return id, true
	}
	return "", false
}

// run performs one transfer, on a slot the dispatcher already took.
func (m *Manager) run(id string) {
	defer func() { <-m.slots }()

	m.mu.Lock()
	item, ok := m.items[id]
	// The transfer may have been canceled, or the manager closed, while it
	// waited for a slot.
	if !ok || m.closed || item.Finished() {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	item.cancel = cancel
	item.State = StateRunning
	item.StartedAt = time.Now()
	transfer := item.Transfer
	sess := item.session
	perform := m.perform
	m.mu.Unlock()
	m.notify()
	defer cancel()

	progress := func(p Progress) {
		m.mu.Lock()
		if it, ok := m.items[id]; ok {
			it.Done = p.Done
			if p.Total > 0 {
				it.Size = p.Total
			}
		}
		m.mu.Unlock()
		m.notify()
	}

	err := perform(ctx, sess, transfer, progress)
	m.mu.Lock()
	item, ok = m.items[id]
	if ok {
		item.FinishedAt = time.Now()
		switch {
		case err == nil:
			item.State = StateDone
			item.Done = item.Size
		case errors.Is(err, ErrCanceled) || errors.Is(err, context.Canceled):
			item.State = StateCanceled
		default:
			item.State = StateFailed
			item.Error = err.Error()
		}
		item.cancel = nil
	}
	m.mu.Unlock()
	m.notify()
}

// performTransfer runs one transfer against the storage endpoint.
func performTransfer(ctx context.Context, sess *Session, t Transfer, progress func(Progress)) error {
	switch t.Kind {
	case KindDownload:
		_, err := sess.Download(ctx, DownloadOptions{
			Bucket: t.Bucket,
			Key:    t.Key,
			Dest:   t.Path,
		}, progress)
		return err
	case KindUpload:
		return sess.Upload(ctx, UploadOptions{
			Bucket: t.Bucket,
			Key:    t.Key,
			Src:    t.Path,
			Size:   t.Size,
		}, progress)
	}
	return errors.New("unknown transfer kind " + string(t.Kind))
}

// Cancel stops a queued or running transfer.
func (m *Manager) Cancel(id string) {
	m.mu.Lock()
	item, ok := m.items[id]
	var cancel context.CancelFunc
	if ok && !item.Finished() {
		cancel = item.cancel
		if cancel == nil {
			// Still queued: mark it canceled so run leaves it alone.
			item.State = StateCanceled
			item.FinishedAt = time.Now()
		}
	}
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.notify()
}

// List returns every transfer, newest last.
func (m *Manager) List() []Transfer {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Transfer, 0, len(m.order))
	for _, id := range m.order {
		if item, ok := m.items[id]; ok {
			out = append(out, item.Transfer)
		}
	}
	return out
}

// Active reports how many transfers are queued or running.
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int
	for _, item := range m.items {
		if !item.Finished() {
			n++
		}
	}
	return n
}

// ClearFinished forgets the transfers that have stopped.
func (m *Manager) ClearFinished() {
	m.mu.Lock()
	kept := m.order[:0]
	for _, id := range m.order {
		if item, ok := m.items[id]; ok && !item.Finished() {
			kept = append(kept, id)
			continue
		}
		delete(m.items, id)
	}
	m.order = kept
	m.mu.Unlock()
	m.notify()
}

// Close stops every transfer and detaches the change callback.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	cancels := make([]context.CancelFunc, 0, len(m.items))
	for _, item := range m.items {
		if item.cancel != nil {
			cancels = append(cancels, item.cancel)
		}
	}
	m.onChange = nil
	close(m.closedCh)
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// DownloadName returns the local file name to save an object under.
func DownloadName(key string) string {
	name := path.Base(key)
	if name == "" || name == "." || name == "/" {
		return "download"
	}
	return filepath.Base(name)
}

// SeedForTest places a transfer in the manager without running it, for
// interface tests and screenshots.
func (m *Manager) SeedForTest(x Transfer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.items == nil {
		m.items = make(map[string]*transferItem)
	}
	m.items[x.ID] = &transferItem{Transfer: x}
	m.order = append(m.order, x.ID)
}
