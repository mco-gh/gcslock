package cloudmutex

import (
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"
	"log"
	"net/http"
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

var (
	mutex                    = &sync.Mutex{}
	client  *http.Client     = nil
	service *storage.Service = nil
	err     error            = nil
)

func init() {
	client, err = google.DefaultClient(context.Background(), scope)
	if err != nil {
		log.Fatalf("Unable to get default client: %v", err)
	}
	service, err = storage.New(client)
	if err != nil {
		log.Fatalf("Unable to create storage service: %v", err)
	}
}

func Lock() {
	//mutex.Lock()
	object := &storage.Object{Name: objectName}
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("Error opening %q: %v", fileName, err)
	}
	if _, err := service.Objects.Insert(bucketName, object).Media(file).Do(); err != nil {
		log.Fatalf("Objects.Insert failed: %v", err)
	}
}

func Unlock() {
	//mutex.Unlock()
	if err := service.Objects.Delete(bucketName, objectName).Do(); err != nil {
		log.Fatalf("Could not delete object: %v\n", err)
	}
}
