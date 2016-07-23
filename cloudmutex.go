package cloudmutex

import (
	"errors"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"
	"log"
	"sync"
	//"os"
)

const (
	//fileName   = "file"
	scope = storage.DevstorageFullControlScope
)

type cloudmutex interface {
	Lock()
	Unlock()
}

type localmutex struct {
	mutex *sync.Mutex
}

type globalmutex struct {
	project string
	bucket  string
	object  string
	service *storage.Service
}

func (m localmutex) Lock() {
	m.mutex.Lock()
}

func (m localmutex) Unlock() {
	m.mutex.Unlock()
}

func (m globalmutex) Lock() {
	object := &storage.Object{Name: m.object}
	//file, err := os.Open(fileName)
	//if err != nil {
	//log.Fatalf("Error opening %q: %v", fileName, err)
	//}
	//if _, err := m.service.Objects.Insert(m.bucket, m.object).Media(file).Do(); err != nil {
	if _, err := m.service.Objects.Insert(m.bucket, object).Do(); err != nil {
		log.Fatalf("Objects.Insert failed: %v", err)
	}
}

func (m globalmutex) Unlock() {
	if err := m.service.Objects.Delete(m.bucket, m.object).Do(); err != nil {
		log.Fatalf("Could not delete object: %v\n", err)
	}
}

func newMutex(scope, project, bucket, object string) (cloudmutex, error) {
	if scope == "local" {
		p := new(localmutex)
		p.mutex = &sync.Mutex{}
		return p, nil
	} else if scope == "" || scope == "global" {
		p := new(globalmutex)
		if p.project != "" {
			p.project = project
		}
		if p.bucket != "" {
			p.bucket = bucket
		}
		if p.object != "" {
			p.object = object
		}
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
