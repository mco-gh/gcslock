package cloudmutex

import (
	"bytes"
	"errors"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"
	"net/http"
	"sync"
	"time"
)

type cloudmutex struct {
	project string
	bucket  string
	object  string
	client  *http.Client
}

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
	url := "https://www.googleapis.com/upload/storage/v1/b/" +
		m.bucket + "/o?uploadType=media&name=" + m.object + "&ifGenerationMatch=0"
	for {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte("1")))
		if err == nil {
			resp, err := m.client.Do(req)
			defer resp.Body.Close()
			if err == nil && resp.StatusCode == 200 {
				return
			}
		}
	}
}

// Unlock waits indefinitely to relinquish a global mutex lock.
func (m cloudmutex) Unlock() {
	url := "https://www.googleapis.com/storage/v1/b/" + m.bucket + "/o/" + m.object
	for {
		req, err := http.NewRequest("DELETE", url, nil)
		if err == nil {
			resp, err := m.client.Do(req)
			defer resp.Body.Close()
			if err == nil {
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
	scope := storage.DevstorageFullControlScope
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
