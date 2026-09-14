package objectstore

import (
	"context"
	"net/http"
	"net/http/httptest"
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
