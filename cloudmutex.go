package cloudmutex

import (
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"
	"log"
	"os"
)

const (
	scope = storage.DevstorageFullControlScope
)

type cloudmutex struct {
	project string
	bucket  string
	object  string
	service *storage.Service
}

func (m cloudmutex) Lock() {
	object := &storage.Object{Name: m.object}
	file, err := os.Open("/dev/null")
	if err != nil {
		log.Fatalf("Error opening /dev/null: %v", err)
		return
	}
	if _, err := m.service.Objects.Insert(m.bucket, object).Media(file).Do(); err != nil {
		log.Fatalf("Objects.Insert failed: %v", err)
	}
}

func (m cloudmutex) Unlock() {
	if err := m.service.Objects.Delete(m.bucket, m.object).Do(); err != nil {
		log.Fatalf("Could not delete object: %v\n", err)
	}
}

func newCloudMutex(project, bucket, object string) (*cloudmutex, error) {
	p := new(cloudmutex)
	if project != "" {
		p.project = project
	}
	if bucket != "" {
		p.bucket = bucket
	}
	if object != "" {
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
