package cloudmutex

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"
)

const (
	project = "marc-general"
	bucket  = "cloudmutex"
	object  = "lock"
)

var (
	limit = 10

	lockHolderMu sync.Mutex
	lockHolder   = -1
)

func TestLock(t *testing.T) {
	// google cloud storage stub
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("r.Method = %q; want POST", r.Method)
		}
		path := "/b/cloudmutex/o"
		if r.URL.Path != path {
			t.Errorf("r.URL.Path = %q; want %q", r.URL.Path, path)
		}
		vals := url.Values{
			"ifGenerationMatch": []string{"0"},
			"name":              []string{"lock"},
			"uploadType":        []string{"media"},
		}
		if !reflect.DeepEqual(r.URL.Query(), vals) {
			t.Errorf("query params = %q; want %q", r.URL.Query(), vals)
		}
	}))
	defer storage.Close()
	storageLockURL = storage.URL

	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Fatal("unable to allocate a cloudmutex global object")
	}
	done := make(chan struct{})
	go func() {
		m.Lock()
		close(done)
	}()
	select {
	case <-time.After(time.Second):
		t.Errorf("m.Lock() took too long to lock")
	case <-done:
		// pass
	}
}

func TestLockRetry(t *testing.T) {
	var (
		retryCount int
		retryLimit = 2
	)
	// google cloud storage stub
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryCount < retryLimit {
			w.WriteHeader(http.StatusInternalServerError)
			retryCount++
		}
	}))
	defer storage.Close()
	storageLockURL = storage.URL

	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Fatal("unable to allocate a cloudmutex global object")
	}
	done := make(chan struct{})
	go func() {
		m.Lock()
		close(done)
	}()
	select {
	case <-time.After(time.Second):
		t.Errorf("m.Lock() took too long to lock")
	case <-done:
		// pass
	}
	if retryCount < retryLimit {
		t.Errorf("retryCount = %d; want %d", retryCount, retryLimit)
	}
}

func TestUnlock(t *testing.T) {
	// google cloud storage stub
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("r.Method = %q; want DELETE", r.Method)
		}
		path := "/b/cloudmutex/o/lock"
		if r.URL.Path != path {
			t.Errorf("r.URL.Path = %q; want %q", r.URL.Path, path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer storage.Close()
	storageUnlockURL = storage.URL

	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Fatal("unable to allocate a cloudmutex global object")
	}
	done := make(chan struct{})
	go func() {
		m.Unlock()
		close(done)
	}()
	select {
	case <-time.After(time.Second):
		t.Errorf("m.Unlock() took too long to lock")
	case <-done:
		// pass
	}
}

func TestUnlockRetry(t *testing.T) {
	var (
		retryCount int
		retryLimit = 2
	)
	// google cloud storage stub
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if retryCount < retryLimit {
			w.WriteHeader(http.StatusInternalServerError)
			retryCount++
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer storage.Close()
	storageUnlockURL = storage.URL

	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Fatal("unable to allocate a cloudmutex global object")
	}
	done := make(chan struct{})
	go func() {
		m.Unlock()
		close(done)
	}()
	select {
	case <-time.After(time.Second):
		t.Errorf("m.Unlock() took too long to lock")
	case <-done:
		// pass
	}
	if retryCount < retryLimit {
		t.Errorf("retryCount = %d; want %d", retryCount, retryLimit)
	}
}

func locker(done chan struct{}, t *testing.T, i int, m sync.Locker) {
	m.Lock()
	lockHolderMu.Lock()
	if lockHolder != -1 {
		t.Errorf("%d trying to lock, but already held by %d",
			i, lockHolder)
	}
	lockHolder = i
	lockHolderMu.Unlock()
	t.Logf("locked by %d", i)
	time.Sleep(5 * time.Millisecond)
	lockHolderMu.Lock()
	lockHolder = -1
	lockHolderMu.Unlock()
	m.Unlock()
	done <- struct{}{}
}

func TestParallel(t *testing.T) {
	storageLockURL = defaultStorageLockURL
	storageUnlockURL = defaultStorageUnlockURL
	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Fatal("unable to allocate a cloudmutex global object")
	}
	done := make(chan struct{}, 1)
	total := 0
	for i := 0; i < limit; i++ {
		total++
		go locker(done, t, i, m)
	}
	for ; total > 0; total-- {
		<-done
	}
}

const timeoutSeconds = 1

// This type is used to mock the sync.Lock interface provided by cloudmutex.
type mockLocker struct {
	wait time.Duration
}

func (l *mockLocker) Lock() {
	time.Sleep(l.wait)
}
func (l *mockLocker) Unlock() {
	time.Sleep(l.wait)
}

func TestLockTimeout(t *testing.T) {
	m := &mockLocker{10 * time.Millisecond}
	if err := Lock(m, time.Millisecond); err == nil {
		t.Errorf("want lock error for Lock(m, 1ms)")
	}
	if err := Lock(m, 100*time.Millisecond); err != nil {
		t.Errorf("Lock(m, 100ms): %v", err)
	}
}

func TestUnlockTimeout(t *testing.T) {
	m := &mockLocker{10 * time.Millisecond}
	if err := Unlock(m, time.Millisecond); err == nil {
		t.Errorf("want unlock error for Unlock(m, 1ms)")
	}
	if err := Unlock(m, 100*time.Millisecond); err != nil {
		t.Errorf("Unlock(m, 100ms): %v", err)
	}
}
