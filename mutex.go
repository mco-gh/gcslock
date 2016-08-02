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

// Package gcslock provides a scalable, distributed mutex that can be used
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

type mutex struct {
	project string
	bucket  string
	object  string
	client  *http.Client
}

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
	go func() {
		l.Lock()
		done <- struct{}{}
	}()
	select {
	case <-done:
		return nil
	case <-time.After(d):
		return errors.New("lock request timed out")
	}
}

// Unlock waits up to duration d for l.Unlock() to succeed.
func Unlock(l sync.Locker, d time.Duration) error {
	done := make(chan struct{}, 1)
	go func() {
		l.Unlock()
		done <- struct{}{}
	}()
	select {
	case <-done:
		return nil
	case <-time.After(d):
		return errors.New("unlock request timed out")
	}
}

// Lock waits indefinitely to acquire a global mutex lock.
func (m mutex) Lock() {
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
				return
			}
		}
		time.Sleep(time.Duration(i) * time.Millisecond)
	}
}

// Unlock waits indefinitely to relinquish a global mutex lock.
func (m mutex) Unlock() {
	url := fmt.Sprintf("%s/b/%s/o/%s?", storageUnlockURL, m.bucket, m.object)
	for i := 1; ; i *= 2 {
		req, err := http.NewRequest("DELETE", url, nil)
		if err == nil {
			res, err := m.client.Do(req)
			if err == nil {
				res.Body.Close()
				if res.StatusCode == 204 {
					return
				}
			}
		}
		time.Sleep(time.Duration(i) * time.Millisecond)
	}
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
	scope := "https://www.googleapis.com/auth/devstorage.full_control"
	client, err := google.DefaultClient(ctx, scope)
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
