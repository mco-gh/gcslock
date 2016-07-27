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
	m.Lock()
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

/* TODO: add testing for timed lock (both success and timeout cases)
func TestLockTimeout(t *testing.T) {
	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Fatal("unable to allocate a cloudmutex global object")
		return
	}
	Lock(m, 3*time.Second)
}
*/

/* TODO: add testing for timed unlock (both success and timeout cases)
func TestUnlockTimeout(t *testing.T) {
	m, err := New(nil, project, bucket, object)
	if err != nil {
		t.Fatal("unable to allocate a cloudmutex global object")
		return
	}
	Unlock(m, 3*time.Second)
}
*/
