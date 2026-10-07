package httpserver

import (
	"context"
	"io"
	"sync"

	"hohoho/internal/config"
	"hohoho/internal/objectstore"
)

// CoreObjectStorage is the hot-swappable S3 handle. The UI saves the Garage/S3
// configuration to runtime settings; PUT /system/configuration opens a new
// client from those settings and swaps it in here without restarting the app.
type CoreObjectStorage struct {
	mu     sync.RWMutex
	client objectstore.Store
}

func NewCoreObjectStorage() *CoreObjectStorage {
	return &CoreObjectStorage{}
}

func (s *CoreObjectStorage) get() objectstore.Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client
}

func (s *CoreObjectStorage) Swap(client objectstore.Store) {
	s.mu.Lock()
	s.client = client
	s.mu.Unlock()
}

func (s *CoreObjectStorage) Ping(ctx context.Context) error {
	client := s.get()
	if client == nil {
		return errObjectStorageUnavailable
	}
	return client.Ping(ctx)
}

func (s *CoreObjectStorage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	client := s.get()
	if client == nil {
		return errObjectStorageUnavailable
	}
	return client.Put(ctx, key, data, contentType)
}

func (s *CoreObjectStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	client := s.get()
	if client == nil {
		return nil, errObjectStorageUnavailable
	}
	return client.Get(ctx, key)
}

func (s *CoreObjectStorage) Delete(ctx context.Context, key string) error {
	client := s.get()
	if client == nil {
		return errObjectStorageUnavailable
	}
	return client.Delete(ctx, key)
}

// OpenCoreObjectStore builds an S3 client from saved runtime settings. An empty
// endpoint means storage is not configured yet and yields no client.
func OpenCoreObjectStore(ctx context.Context, cfg config.Runtime) (objectstore.Store, error) {
	if cfg.S3Endpoint == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" || cfg.S3Bucket == "" {
		return nil, nil
	}
	return objectstore.Open(ctx, objectstore.Config{
		Endpoint: cfg.S3Endpoint, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey,
		Bucket: cfg.S3Bucket, Region: cfg.S3Region, UseTLS: cfg.S3UseTLS,
		PathStyle: cfg.S3PathStyle, CreateBucket: cfg.S3CreateBucket,
	})
}
