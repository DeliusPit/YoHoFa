package objectstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Store interface {
	Ping(context.Context) error
	Put(context.Context, string, []byte, string) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

type MinIO struct {
	mu           sync.Mutex
	client       *minio.Client
	endpoint     string
	accessKey    string
	secretKey    string
	bucket       string
	useSSL       bool
	region       string
	pathStyle    bool
	createBucket bool
}

type Config struct {
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Bucket       string
	UseTLS       bool
	Region       string
	PathStyle    bool
	CreateBucket bool
}

func Open(ctx context.Context, cfg Config) (*MinIO, error) {
	store := &MinIO{endpoint: cfg.Endpoint, accessKey: cfg.AccessKey, secretKey: cfg.SecretKey, bucket: cfg.Bucket, useSSL: cfg.UseTLS, region: cfg.Region, pathStyle: cfg.PathStyle, createBucket: cfg.CreateBucket}
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		return store, fmt.Errorf("S3 configuration is incomplete")
	}
	return store, store.checkBucket(ctx)
}

func (s *MinIO) Ping(ctx context.Context) error {
	return s.checkBucket(ctx)
}

func (s *MinIO) Put(ctx context.Context, objectKey string, data []byte, contentType string) error {
	client, err := s.readyClient(ctx)
	if err != nil {
		return err
	}
	_, err = client.PutObject(ctx, s.bucket, objectKey, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *MinIO) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	client, err := s.readyClient(ctx)
	if err != nil {
		return nil, err
	}
	object, err := client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := object.Stat(); err != nil {
		object.Close()
		return nil, err
	}
	return object, nil
}

func (s *MinIO) Delete(ctx context.Context, objectKey string) error {
	client, err := s.readyClient(ctx)
	if err != nil {
		return err
	}
	return client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
}

func (s *MinIO) readyClient(ctx context.Context) (*minio.Client, error) {
	if err := s.checkBucket(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client, nil
}

func (s *MinIO) checkBucket(ctx context.Context) error {
	client, err := s.getClient()
	if err != nil {
		return err
	}
	exists, err := client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if !s.createBucket {
			return fmt.Errorf("S3 bucket is unavailable")
		}
		if err := client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: s.region}); err != nil {
			exists, checkErr := client.BucketExists(ctx, s.bucket)
			if checkErr != nil || !exists {
				return err
			}
		}
	}
	return nil
}

func (s *MinIO) getClient() (*minio.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		return s.client, nil
	}
	if s.endpoint == "" || s.accessKey == "" || s.secretKey == "" || s.bucket == "" {
		return nil, fmt.Errorf("S3 configuration is incomplete")
	}
	bucketLookup := minio.BucketLookupDNS
	if s.pathStyle {
		bucketLookup = minio.BucketLookupPath
	}
	client, err := minio.New(s.endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(s.accessKey, s.secretKey, ""),
		Secure:       s.useSSL,
		Region:       s.region,
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, err
	}
	s.client = client
	return client, nil
}
