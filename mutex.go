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

// Package gcslock is a scalable, distributed mutex that can be used
// to serialize computations anywhere on the global internet.
package gcslock

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
)

const (
	defaultStorageLockURL   = "https://www.googleapis.com/upload/storage/v1"
	defaultStorageUnlockURL = "https://www.googleapis.com/storage/v1"
)

var (
	// These vars are used in the requests below. Having separate default
	// values makes it easy to reset the standard config during testing.
	storageLockURL   = defaultStorageLockURL
	storageUnlockURL = defaultStorageUnlockURL
)

// Lock waits up to duration d for l.Lock() to succeed.
func Lock(l sync.Locker, d time.Duration) error {
	done := make(chan struct{}, 1)
	m, ok := l.(*mutex)
	var err error = nil
	// if l is a mutex, lock with a timeout context.
	if ok {
		ctx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()
		go func() {
			err = m.lock(ctx)
			done <- struct{}{}
		}()
	} else {
		go func() {
			l.Lock()
			done <- struct{}{}
		}()
	}
	select {
	case <-done:
		return err
	case <-time.After(d):
		return errors.New("lock request timed out")
	}
}

// Unlock waits up to duration d for l.Unlock() to succeed.
func Unlock(l sync.Locker, d time.Duration) error {
	done := make(chan struct{}, 1)
	m, ok := l.(*mutex)
	var err error = nil
	// if l is a mutex, unlock with a timeout context.
	if ok {
		ctx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()
		go func() {
			err = m.unlock(ctx)
			done <- struct{}{}
		}()
	} else {
		go func() {
			l.Unlock()
			done <- struct{}{}
		}()
	}
	select {
	case <-done:
		return err
	case <-time.After(d):
		return errors.New("unlock request timed out")
	}
}

type mutex struct {
	project string
	bucket  string
	object  string
	client  *http.Client
}

// Lock waits indefinitely to acquire a mutex.
func (m *mutex) Lock() {
	m.lock(context.Background())
}

// Private method that waits indefinitely to acquire a mutex
// with aggregate timeout governed by passed context.
func (m *mutex) lock(ctx context.Context) error {
	q := url.Values{
		"name":              {m.object},
		"uploadType":        {"media"},
		"ifGenerationMatch": {"0"},
	}
	url := fmt.Sprintf("%s/b/%s/o?%s", storageLockURL, m.bucket, q.Encode())
	for i := 1; ; i *= 2 {
		res, err := m.client.Post(url, "plain/text", bytes.NewReader([]byte("1")))
		if err == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				return nil
			}
		}
		select {
		case <-time.After(time.Duration(i) * time.Millisecond):
			continue
		case <-ctx.Done():
			return errors.New("lock request timed out")
		}
	}
}

// Unlock waits indefinitely to release a mutex.
func (m *mutex) Unlock() {
	m.unlock(context.Background())
}

// Private method that waits indefinitely to release a mutex
// with aggregate timeout governed by passed context.
func (m *mutex) unlock(ctx context.Context) error {
	url := fmt.Sprintf("%s/b/%s/o/%s?", storageUnlockURL, m.bucket, m.object)
	for i := 1; ; i *= 2 {
		req, err := http.NewRequest("DELETE", url, nil)
		if err == nil {
			res, err := m.client.Do(req)
			if err == nil {
				res.Body.Close()
				if res.StatusCode == 204 {
					return nil
				}
			}
		}
		select {
		case <-time.After(time.Duration(i) * time.Millisecond):
			continue
		case <-ctx.Done():
			return errors.New("unlock request timed out")
		}
	}
}

// httpClient is overwritten in tests
var httpClient = func(ctx context.Context) (*http.Client, error) {
	const scope = "https://www.googleapis.com/auth/devstorage.full_control"
	return google.DefaultClient(ctx, scope)
}

// New creates a GCS-based sync.Locker.
// It uses Application Default Credentials to make authenticated requests
// to Google Cloud Storage. See the DefaultClient function of the
// golang.org/x/oauth2/google package for App Default Credentials details.
//
// If ctx argument is nil, context.Background is used.
//
func New(ctx context.Context, project, bucket, object string) (sync.Locker, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := httpClient(ctx)
	if err != nil {
		return nil, err
	}
	m := &mutex{
		project: project,
		bucket:  bucket,
		object:  object,
		client:  client,
	}
	return m, nil
}
