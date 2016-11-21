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

// ContextLocker provides an extension of the sync.Locker interface.
type ContextLocker interface {
	sync.Locker
	ContextLock(context.Context) error
	ContextUnlock(context.Context) error
}

type mutex struct {
	bucket string
	object string
	client *http.Client
}

var _ ContextLocker = (*mutex)(nil)

// Lock waits indefinitely to acquire a mutex.
func (m *mutex) Lock() {
	m.ContextLock(context.Background())
}

// ContextLock waits indefinitely to acquire a mutex with timeout
// governed by passed context.
func (m *mutex) ContextLock(ctx context.Context) error {
	q := url.Values{
		"name":              {m.object},
		"uploadType":        {"media"},
		"ifGenerationMatch": {"0"},
	}
	url := fmt.Sprintf("%s/b/%s/o?%s", storageLockURL, m.bucket, q.Encode())
	// NOTE: ctx deadline/timeout and backoff are independent. The former is
	// an aggregate timeout and the latter is a per loop iteration delay.
	backoff := 10 * time.Millisecond
	for {
		req, err := http.NewRequest("POST", url, bytes.NewReader([]byte("1")))
		if err != nil {
			// Likely malformed URL - retry won't fix so return.
			return err
		}
		req.Header.Set("content-type", "text/plain")
		req = req.WithContext(ctx)
		res, err := m.client.Do(req)
		if err == nil {
			res.Body.Close()
			if res.StatusCode == 200 {
				return nil
			}
		}
		select {
		case <-time.After(backoff):
			backoff *= 2
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Unlock waits indefinitely to release a mutex.
func (m *mutex) Unlock() {
	m.ContextUnlock(context.Background())
}

// ContextUnlock waits indefinitely to release a mutex with timeout
// governed by passed context.
func (m *mutex) ContextUnlock(ctx context.Context) error {
	url := fmt.Sprintf("%s/b/%s/o/%s?", storageUnlockURL, m.bucket, m.object)
	// NOTE: ctx deadline/timeout and backoff are independent. The former is
	// an aggregate timeout and the latter is a per loop iteration delay.
	backoff := 10 * time.Millisecond
	for {
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			// Likely malformed URL - retry won't fix so return.
			return err
		}
		req = req.WithContext(ctx)
		res, err := m.client.Do(req)
		if err == nil {
			res.Body.Close()
			if res.StatusCode == 204 {
				return nil
			}
		}
		select {
		case <-time.After(backoff):
			backoff *= 2
			continue
		case <-ctx.Done():
			return ctx.Err()
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
func New(ctx context.Context, bucket, object string) (ContextLocker, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client, err := httpClient(ctx)
	if err != nil {
		return nil, err
	}
	m := &mutex{
		bucket: bucket,
		object: object,
		client: client,
	}
	return m, nil
}
