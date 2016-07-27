package cloudmutex

import (
	"net/http"
	"net/http/httptest"
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
			t.Errorf("unlock request method (%s) != POST", r.Method)
		}
		if r.URL.String() != "/b/cloudmutex/o?ifGenerationMatch=0&name=lock&uploadType=media" {
			t.Errorf("unexpected URL sent for lock request: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer storage.Close()
	storageLockURL = storage.URL

	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Errorf("unable to allocate a cloudmutex global object")
		return
	}
	m.Lock()
}

func TestUnlock(t *testing.T) {
	// google cloud storage stub
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("unlock request method (%s) != DELETE", r.Method)
		}
		if r.URL.String() != "/b/cloudmutex/o/lock" {
			t.Errorf("unexpected URL sent for unlock request: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer storage.Close()
	storageUnlockURL = storage.URL

	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Errorf("unable to allocate a cloudmutex global object")
		return
	}
	m.Unlock()
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
	time.Sleep(10 * time.Millisecond)
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
		t.Errorf("unable to allocate a cloudmutex global object")
		return
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

/* TODO: add testing for timed lock (both success and timeout cases)
func TestLockTimeout(t *testing.T) {
	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Errorf("unable to allocate a cloudmutex global object")
		return
	}
	Lock(m, 3*time.Second)
}
*/

/* TODO: add testing for timed unlock (both success and timeout cases)
func TestUnlockTimeout(t *testing.T) {
	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Errorf("unable to allocate a cloudmutex global object")
		return
	}
	Unlock(m, 3*time.Second)
}
*/
