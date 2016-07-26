package cloudmutex

import (
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
	m.Unlock()
	lockHolderMu.Lock()
	lockHolder = -1
	lockHolderMu.Unlock()
	done <- struct{}{}
}

func TestParallel(t *testing.T) {
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
