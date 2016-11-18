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

package gcslock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"
)

// make sure mutex pointer satisfies sync.Locker
var _ sync.Locker = &mutex{}

func init() {
	httpClient = func(context.Context) (*http.Client, error) { return http.DefaultClient, nil }
}

func TestLock(t *testing.T) {
	// google cloud storage stub
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("r.Method = %q; want POST", r.Method)
		}
		path := "/b/gcslock/o"
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
	m, err := New(nil, "stub", "gcslock", "lock")
	if err != nil {
		t.Fatal(err)
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

	m, err := New(nil, "stub", "gcslock", "lock")
	if err != nil {
		t.Fatal(err)
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
		path := "/b/gcslock/o/lock"
		if r.URL.Path != path {
			t.Errorf("r.URL.Path = %q; want %q", r.URL.Path, path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer storage.Close()
	storageUnlockURL = storage.URL

	m, err := New(nil, "stub", "gcslock", "lock")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		m.Unlock()
		close(done)
	}()
	select {
	case <-time.After(time.Second):
		t.Errorf("m.Unlock() took too long to unlock")
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

	m, err := New(nil, "stub", "gcslock", "lock")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		m.Unlock()
		close(done)
	}()
	select {
	case <-time.After(time.Second):
		t.Errorf("m.Unlock() took too long to unlock")
	case <-done:
		// pass
	}
	if retryCount < retryLimit {
		t.Errorf("retryCount = %d; want %d", retryCount, retryLimit)
	}
}

type mockContextLocker struct {
	mutex
	wait time.Duration
}

func (l *mockContextLocker) Lock() {
	time.Sleep(l.wait)
}

func (l *mockContextLocker) Unlock() {
	time.Sleep(l.wait)
}

func TestLockTimeout(t *testing.T) {
	mut, err := New(nil, "stub", "gcslock", "lock")
	if err != nil {
		t.Fatal("unable to allocate a gcslock.mutex object")
	}
	m := &mockContextLocker{*mut, 10 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := m.ContextLock(ctx); err == nil {
		t.Errorf("want lock timeout for ContextLock w/ 1ms timeout")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err = m.ContextLock(ctx); err != nil {
		t.Errorf("ContextLock w/ 100ms timeout: %v", err)
	}
}

func TestUnlockTimeout(t *testing.T) {
	mut, err := New(nil, "stub", "gcslock", "lock")
	if err != nil {
		t.Fatal("unable to allocate a gcslock.mutex object")
	}
	m := &mockContextLocker{*mut, 10 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := m.ContextUnlock(ctx); err == nil {
		t.Errorf("want unlock timeout for ContextUnlock w/ 1ms timeout")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err = m.ContextUnlock(ctx); err != nil {
		t.Errorf("ContextUnlock w/ 100ms timeout: %v", err)
	}
}
