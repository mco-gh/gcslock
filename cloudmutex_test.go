package cloudmutex

import (
	"sync"
	"testing"
	"time"
)

const (
	PROJECT = "marc-general"
	BUCKET  = "cloudmutex"
	OBJECT  = "lock"
)

var (
	limit        = 1
	lock_held_by = -1
)

func locker(done chan bool, t *testing.T, i int, m sync.Locker) {
	m.Lock()
	if lock_held_by != -1 {
		t.Errorf("%d trying to lock, but already held by %d",
			i, lock_held_by)
	}
	lock_held_by = i
	t.Logf("locked by %d", i)
	time.Sleep(10 * time.Millisecond)
	lock_held_by = -1
	m.Unlock()
	done <- true
}

func TestParallelLocal(t *testing.T) {
	m := &sync.Mutex{}
	runParallelTest(t, m)
}

func TestParallelGlobal(t *testing.T) {
	m, err := New(nil, PROJECT, BUCKET, OBJECT)
	if err != nil {
		t.Errorf("unable to allocate a cloudmutex global object")
		return
	}
	runParallelTest(t, m)
}

func runParallelTest(t *testing.T, m sync.Locker) {
	done := make(chan bool, 1)
	total := 0
	for i := 0; i < limit; i++ {
		total++
		go locker(done, t, i, m)
	}
	for ; total > 0; total-- {
		<-done
	}
}

func TestLockTimeout(t *testing.T) {
	m, err := New(nil, PROJECT, BUCKET, OBJECT)
	if err != nil {
		t.Errorf("unable to allocate a cloudmutex global object")
		return
	}
	TimedLock(m, 3*time.Second)
}

func TestUnlockTimeout(t *testing.T) {
	m, err := New(nil, PROJECT, BUCKET, OBJECT)
	if err != nil {
		t.Errorf("unable to allocate a cloudmutex global object")
		return
	}
	TimedUnlock(m, 3*time.Second)
}
