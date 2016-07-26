package cloudmutex

import (
	"bytes"
	"errors"
	// TODO: remove fmt use when done debugging
	"fmt"
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

// Lock will wait up to duruation d for l.Lock() to succeed.
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

// Unlock will wait up to duruation d for l.Unlock() to succeed.
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

// The Lock method waits indefinitely to acquire a global mutex lock.
func (m cloudmutex) Lock() {
	url := "https://www.googleapis.com/upload/storage/v1/b/" +
		m.bucket + "/o?uploadType=media&name=" + m.object + "&ifGenerationMatch=0"
	for {
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte("1")))
		resp, err := m.client.Do(req)
		fmt.Printf("resp:%v, err:%v\n", resp.StatusCode, err)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return
		}
	}
}

// The Unlock method waits indefinitely to relinquish a global mutex lock.
func (m cloudmutex) Unlock() {
	url := "https://www.googleapis.com/storage/v1/b/" +
		m.bucket + "/o/" + m.object
	for {
		req, _ := http.NewRequest("DELETE", url, nil)
		resp, err := m.client.Do(req)
		fmt.Printf("resp:%v, err:%v\n", resp.StatusCode, err)
		if err == nil {
			resp.Body.Close()
			return
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
