package objectstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testObjectKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestFilesystemRoundTripOverwriteAndDelete(t *testing.T) {
	store := newTestFilesystem(t)
	ctx := context.Background()

	if err := store.Put(ctx, testObjectKey, []byte("first"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	assertObjectContents(t, store, testObjectKey, "first")

	if err := store.Put(ctx, testObjectKey, []byte("second"), "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	assertObjectContents(t, store, testObjectKey, "second")

	if err := store.Delete(ctx, testObjectKey); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, testObjectKey); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
	reader, err := store.Get(ctx, testObjectKey)
	if reader != nil {
		reader.Close()
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete error = %v, want ErrNotFound", err)
	}
}

func TestFilesystemRejectsInvalidKeysBeforeUsingPaths(t *testing.T) {
	store := newTestFilesystem(t)
	invalid := []string{
		"",
		"abc",
		strings.Repeat("a", 63),
		strings.Repeat("a", 65),
		strings.Repeat("A", 64),
		strings.Repeat("g", 64),
		"../../../../../../../../../../../../../../../../../../etc/passwd",
		strings.Repeat("a", 31) + "/" + strings.Repeat("b", 32),
	}
	for _, key := range invalid {
		t.Run(key, func(t *testing.T) {
			if err := store.Put(context.Background(), key, nil, ""); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Put error = %v, want ErrInvalidKey", err)
			}
			if _, err := store.Get(context.Background(), key); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Get error = %v, want ErrInvalidKey", err)
			}
			if err := store.Delete(context.Background(), key); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Delete error = %v, want ErrInvalidKey", err)
			}
		})
	}
}

func TestFilesystemRejectsSymlinks(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		parent := t.TempDir()
		target := filepath.Join(parent, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(parent, "root")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, err := NewFilesystem(link); err == nil {
			t.Fatal("NewFilesystem accepted a symlink root")
		}
	})

	t.Run("shard", func(t *testing.T) {
		store := newTestFilesystem(t)
		objects := filepath.Join(store.root, "objects")
		if err := os.Mkdir(objects, 0o700); err != nil {
			t.Fatal(err)
		}
		target := t.TempDir()
		if err := os.Symlink(target, filepath.Join(objects, testObjectKey[:2])); err != nil {
			t.Fatal(err)
		}
		if err := store.Put(context.Background(), testObjectKey, []byte("data"), ""); err == nil {
			t.Fatal("Put accepted a symlink shard")
		}
	})

	t.Run("final", func(t *testing.T) {
		store := newTestFilesystem(t)
		if err := store.Put(context.Background(), testObjectKey, []byte("data"), ""); err != nil {
			t.Fatal(err)
		}
		path := filesystemObjectPath(store, testObjectKey)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), "target")
		if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Get(context.Background(), testObjectKey); err == nil {
			t.Fatal("Get accepted a symlink object")
		}
		if err := store.Put(context.Background(), testObjectKey, []byte("replacement"), ""); err == nil {
			t.Fatal("Put accepted a symlink object")
		}
		if err := store.Delete(context.Background(), testObjectKey); err == nil {
			t.Fatal("Delete accepted a symlink object")
		}
		contents, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		if string(contents) != "outside" {
			t.Fatalf("symlink target contents = %q", contents)
		}
	})
}

func TestFilesystemModes(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "store")
	store, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), testObjectKey, []byte("data"), ""); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		root,
		filepath.Join(root, "objects"),
		filepath.Join(root, "objects", testObjectKey[:2]),
		filepath.Join(root, "objects", testObjectKey[:2], testObjectKey[2:4]),
	} {
		assertMode(t, path, 0o700)
	}
	assertMode(t, filesystemObjectPath(store, testObjectKey), 0o600)
}

func TestFilesystemPingUsesRootProbeAndCleansUp(t *testing.T) {
	store := newTestFilesystem(t)
	if err := store.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(store.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("Ping left entries in root: %v", entries)
	}

	readOnlyRoot := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(readOnlyRoot, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	store.root = readOnlyRoot
	if err := store.Ping(context.Background()); err == nil {
		t.Fatal("Ping succeeded when root was not a directory")
	}
}

func TestFilesystemCancelledContextDoesNotPublish(t *testing.T) {
	store := newTestFilesystem(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Put(ctx, testObjectKey, []byte("data"), ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("Put error = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(filesystemObjectPath(store, testObjectKey)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("object stat error = %v, want not exist", err)
	}
	if err := store.Ping(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Ping error = %v, want context.Canceled", err)
	}
}

func TestNewFilesystemRequiresAbsoluteRoot(t *testing.T) {
	if _, err := NewFilesystem("relative"); err == nil {
		t.Fatal("NewFilesystem accepted a relative root")
	}
}

func newTestFilesystem(t *testing.T) *Filesystem {
	t.Helper()
	store, err := NewFilesystem(filepath.Join(t.TempDir(), "objects-root"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func assertObjectContents(t *testing.T, store *Filesystem, key, want string) {
	t.Helper()
	reader, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("object contents = %q, want %q", data, want)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}

func filesystemObjectPath(store *Filesystem, key string) string {
	return filepath.Join(store.root, "objects", key[:2], key[2:4], key)
}
