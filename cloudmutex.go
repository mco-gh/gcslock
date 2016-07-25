package cloudmutex

import (
	"bytes"
	"errors"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"
	"sync"
	"time"
)

type cloudmutex struct {
	project string
	bucket  string
	object  string
	service *storage.Service
}

// TimedLock will wait up to duruation d for l.Lock() to succeed.
func TimedLock(l sync.Locker, d time.Duration) error {
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

// TimedUnlock will wait up to duruation d for l.Unlock() to succeed.
func TimedUnlock(l sync.Locker, d time.Duration) error {
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

func (m cloudmutex) Lock() {
	object := &storage.Object{Name: m.object}
	for {
		_, err := m.service.Objects.Insert(m.bucket, object).Media(bytes.NewReader([]byte("1"))).Do()
		if err == nil {
			return
		}
	}
}

func (m cloudmutex) Unlock() {
	for {
		err := m.service.Objects.Delete(m.bucket, m.object).Do()
		if err == nil {
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
	service, err := storage.New(client)
	if err != nil {
		return nil, err
	}
	m := &cloudmutex{
		project: project,
		bucket:  bucket,
		object:  object,
		service: service,
	}
	return m, nil
}
