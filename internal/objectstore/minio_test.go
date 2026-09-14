package objectstore

import (
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOpenDoesNotCreateMissingBucketWhenDisabled(t *testing.T) {
	var createRequests atomic.Int32
	var pathStyleRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/documents/" || r.URL.Path == "/documents" {
			pathStyleRequests.Add(1)
		}
		if r.Method == http.MethodPut {
			createRequests.Add(1)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<Error><Code>NoSuchBucket</Code><Message>missing</Message></Error>`))
	}))
	defer server.Close()

	_, err := Open(context.Background(), Config{
		Endpoint:  strings.TrimPrefix(server.URL, "http://"),
		AccessKey: "access",
		SecretKey: "secret",
		Bucket:    "documents",
		Region:    "garage",
		PathStyle: true,
	})
	if err == nil {
		t.Fatal("Open succeeded with a missing bucket")
	}
	if createRequests.Load() != 0 {
		t.Fatalf("Open made %d bucket creation requests with creation disabled", createRequests.Load())
	}
	if pathStyleRequests.Load() == 0 {
		t.Fatal("Open did not use path-style bucket lookup")
	}
}

func TestLiveS3(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	bucket := os.Getenv("S3_BUCKET")
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY and S3_BUCKET are required")
	}
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatal(err)
	}
	const hex = "0123456789abcdef"
	encoded := make([]byte, 64)
	for i, value := range keyBytes {
		encoded[i*2] = hex[value>>4]
		encoded[i*2+1] = hex[value&0x0f]
	}
	objectKey := string(encoded)

	store, err := Open(context.Background(), Config{
		Endpoint:     endpoint,
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		Bucket:       bucket,
		UseTLS:       os.Getenv("S3_USE_TLS") != "false",
		Region:       envOr("S3_REGION", "garage"),
		PathStyle:    os.Getenv("S3_PATH_STYLE") != "false",
		CreateBucket: os.Getenv("S3_CREATE_BUCKET") == "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Delete(context.Background(), objectKey)
	if err := store.Put(context.Background(), objectKey, []byte("garage-check"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	reader, err := store.Get(context.Background(), objectKey)
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	if err := store.Delete(context.Background(), objectKey); err != nil {
		t.Fatal(err)
	}
}

func envOr(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
