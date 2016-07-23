package cloudmutex

import (
	"errors"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"
	"log"
	"os"
	"sync"
)

const (
	fileName   = "file"
	objectName = "object"
	scope      = storage.DevstorageFullControlScope
	projectID  = "id"
	bucketName = "bucket"
)

type cloudmutex interface {
	Lock()
	Unlock()
}

type localmutex struct {
	mutex *sync.Mutex
}

type globalmutex struct {
	service *storage.Service
}

func (m localmutex) Lock() {
	m.mutex.Lock()
}

func (m localmutex) Unlock() {
	m.mutex.Unlock()
}

func (m globalmutex) Lock() {
	object := &storage.Object{Name: objectName}
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("Error opening %q: %v", fileName, err)
	}
	if _, err := m.service.Objects.Insert(bucketName, object).Media(file).Do(); err != nil {
		log.Fatalf("Objects.Insert failed: %v", err)
	}
}

func (m globalmutex) Unlock() {
	if err := m.service.Objects.Delete(bucketName, objectName).Do(); err != nil {
		log.Fatalf("Could not delete object: %v\n", err)
	}
}

func newMutex(scope string) (cloudmutex, error) {
	if scope == "local" {
		p := new(localmutex)
		p.mutex = &sync.Mutex{}
		return p, nil
	} else if scope == "" || scope == "global" {
		p := new(globalmutex)
		client, err := google.DefaultClient(context.Background(), scope)
		if err != nil {
			log.Fatalf("Unable to get default client: %v", err)
		}
		service, err := storage.New(client)
		if err != nil {
			log.Fatalf("Unable to create storage service: %v", err)
		}
		p.service = service
		return p, nil
	}
	return nil, errors.New("invalid scope argument: " + scope)
}
