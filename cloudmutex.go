package cloudmutex

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

type cloudmutex struct {
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
	storageLockURL   = defaultStorageLockURL
	storageUnlockURL = defaultStorageUnlockURL
)

// Lock waits up to duruation d for l.Lock() to succeed.
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

// Unlock waits up to duruation d for l.Unlock() to succeed.
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
func (m cloudmutex) Lock() {
	q := url.Values{
		"name":              {m.object},
		"uploadType":        {"media"},
		"ifGenerationMatch": {"0"},
	}
	url := fmt.Sprintf("%s/b/%s/o?%s", storageLockURL, m.bucket, q.Encode())
	for {
		res, err := m.client.Post(url, "plain/text", bytes.NewReader([]byte("1")))
		if err != nil {
			continue
		}
		res.Body.Close()
		if res.StatusCode == 200 {
			return
		}
	}
}

// Unlock waits indefinitely to relinquish a global mutex lock.
func (m cloudmutex) Unlock() {
	url := fmt.Sprintf("%s/b/%s/o/%s?", storageUnlockURL, m.bucket, m.object)
	for {
		for {
			req, err := http.NewRequest("DELETE", url, nil)
			if err != nil {
				continue
			}
			res, err := m.client.Do(req)
			if err != nil {
				continue
			}
			res.Body.Close()
			if res.StatusCode == 204 {
				return
			}
		}
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
	m := &cloudmutex{
		project: project,
		bucket:  bucket,
		object:  object,
		client:  client,
	}
	return m, nil
}
