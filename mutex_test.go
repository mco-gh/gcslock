package cloudmutex

import (
	"testing"
	"time"
)

var (
	limit        = 100
	lock_held_by = -1
)

func locker(done chan bool, t *testing.T, i int) {
	Lock()
	if lock_held_by != -1 {
		t.Errorf("%d trying to lock, but already held by %d",
			i, lock_held_by)
	}
	lock_held_by = i
	t.Logf("locked by %d", i)
	time.Sleep(10 * time.Millisecond)
	lock_held_by = -1
	Unlock()
	done <- true
}

func TestParallel(t *testing.T) {
	done := make(chan bool, 1)
	total := 0
	for i := 0; i < limit; i++ {
		total++
		go locker(done, t, i)
	}
	for ; total > 0; total-- {
		<-done
	}
}
