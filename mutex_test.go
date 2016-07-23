package cloudmutex

import (
	"testing"
	"time"
)

var (
	limit        = 1
	lock_held_by = -1
)

func locker(done chan bool, t *testing.T, i int, m cloudmutex) {
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
	m, err := newMutex("local", "", "", "")
	if err != nil {
		t.Errorf("unable to allocate a cloudmutex local object")
		return
	}
	runParallelTest(t, m)
}

func TestParallelGlobal(t *testing.T) {
	m, err := newMutex("global", "marc-general", "cloudmutex", "lock")
	if err != nil {
		t.Errorf("unable to allocate a cloudmutex global object")
		return
	}
	runParallelTest(t, m)
}

func runParallelTest(t *testing.T, m cloudmutex) {
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
