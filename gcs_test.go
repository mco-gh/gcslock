// Copyright 2016 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//+build integration

package gcslock

import (
	"sync"
	"testing"
	"time"
)

const (
	project = "marc-general"
	bucket  = "gcslock"
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
		t.Fatal("unable to allocate a gcslock.mutex object")
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
