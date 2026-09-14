//go:build windows

package objectstore

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type Filesystem struct {
	root string
}

var _ Store = (*Filesystem)(nil)

func NewFilesystem(root string) (*Filesystem, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("filesystem object store root must be absolute")
	}
	root = filepath.Clean(root)
	if filepath.Dir(root) == root {
		return nil, fmt.Errorf("filesystem object store root must be a dedicated directory")
	}
	if err := os.Mkdir(root, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("create filesystem object store root: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect filesystem object store root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("filesystem object store root must be a real directory")
	}
	return &Filesystem{root: root}, nil
}

func (s *Filesystem) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := randomTempName(".probe-")
	if err != nil {
		return err
	}
	path := filepath.Join(s.root, name)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create storage probe: %w", err)
	}
	removed := false
	defer func() {
		_ = file.Close()
		if !removed {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write([]byte{0}); err != nil {
		return fmt.Errorf("write storage probe: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync storage probe: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close storage probe: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove storage probe: %w", err)
	}
	removed = true
	return nil
}

func (s *Filesystem) Put(ctx context.Context, objectKey string, data []byte, contentType string) error {
	_ = contentType
	if err := validateObjectKey(objectKey); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	shard, finalPath, err := s.objectPaths(objectKey, true)
	if err != nil {
		return err
	}
	if err := rejectNonRegular(finalPath); err != nil {
		return err
	}
	tempName, err := randomTempName(".tmp-")
	if err != nil {
		return err
	}
	tempPath := filepath.Join(shard, tempName)
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary object: %w", err)
	}
	published := false
	defer func() {
		_ = file.Close()
		if !published {
			_ = os.Remove(tempPath)
		}
	}()
	for len(data) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunk := data
		if len(chunk) > 128*1024 {
			chunk = chunk[:128*1024]
		}
		n, err := file.Write(chunk)
		if err != nil {
			return fmt.Errorf("write temporary object: %w", err)
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync temporary object: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary object: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := rejectNonRegular(finalPath); err != nil {
		return err
	}
	if _, err := os.Lstat(finalPath); err == nil {
		if err := os.Remove(finalPath); err != nil {
			return fmt.Errorf("replace object: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect object path: %w", err)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("publish object: %w", err)
	}
	published = true
	return nil
}

func (s *Filesystem) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	if err := validateObjectKey(objectKey); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_, path, err := s.objectPaths(objectKey, false)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, objectNotFound(objectKey)
		}
		return nil, err
	}
	if err := requireRegular(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, objectNotFound(objectKey)
		}
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, objectNotFound(objectKey)
		}
		return nil, fmt.Errorf("open object: %w", err)
	}
	return file, nil
}

func (s *Filesystem) Delete(ctx context.Context, objectKey string) error {
	if err := validateObjectKey(objectKey); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, path, err := s.objectPaths(objectKey, false)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := requireRegular(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func (s *Filesystem) objectPaths(key string, create bool) (string, string, error) {
	shard := filepath.Join(s.root, "objects", key[:2], key[2:4])
	if create {
		if err := os.MkdirAll(shard, 0o700); err != nil {
			return "", "", fmt.Errorf("create object directory: %w", err)
		}
	} else if info, err := os.Stat(shard); err != nil {
		return "", "", err
	} else if !info.IsDir() {
		return "", "", fmt.Errorf("object directory is not a directory")
	}
	return shard, filepath.Join(shard, key), nil
}

func requireRegular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("object path is not a regular file")
	}
	return nil
}

func rejectNonRegular(path string) error {
	if err := requireRegular(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func randomTempName(prefix string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%x", prefix, random), nil
}
