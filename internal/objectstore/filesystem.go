//go:build !windows

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

	"golang.org/x/sys/unix"
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
	created := false
	if err := os.Mkdir(root, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("create filesystem object store root: %w", err)
	} else if err == nil {
		created = true
	}

	store := &Filesystem{root: root}
	fd, err := store.openRoot()
	if err != nil {
		return nil, err
	}
	if created {
		if err := unix.Fchmod(fd, 0o700); err != nil {
			unix.Close(fd)
			return nil, fmt.Errorf("set filesystem object store root permissions: %w", err)
		}
	}
	if err := unix.Close(fd); err != nil {
		return nil, fmt.Errorf("close filesystem object store root: %w", err)
	}
	return store, nil
}

func (s *Filesystem) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rootFD, err := s.openRoot()
	if err != nil {
		return err
	}
	defer unix.Close(rootFD)

	name, fd, err := createTempAt(rootFD, ".probe-")
	if err != nil {
		return fmt.Errorf("create storage probe: %w", err)
	}
	probe := os.NewFile(uintptr(fd), name)
	removed := false
	defer func() {
		_ = probe.Close()
		if !removed {
			_ = unix.Unlinkat(rootFD, name, 0)
		}
	}()

	if _, err := probe.Write([]byte{0}); err != nil {
		return fmt.Errorf("write storage probe: %w", err)
	}
	if err := probe.Sync(); err != nil {
		return fmt.Errorf("sync storage probe: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close storage probe: %w", err)
	}
	if err := unix.Unlinkat(rootFD, name, 0); err != nil {
		return fmt.Errorf("remove storage probe: %w", err)
	}
	removed = true
	if err := unix.Fsync(rootFD); err != nil {
		return fmt.Errorf("sync filesystem object store root: %w", err)
	}
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

	shardFD, closeDirs, err := s.openShard(objectKey, true)
	if err != nil {
		return err
	}
	defer closeDirs()

	if _, err := regularEntryAt(shardFD, objectKey); err != nil {
		return err
	}

	tempName, tempFD, err := createTempAt(shardFD, ".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary object: %w", err)
	}
	temp := os.NewFile(uintptr(tempFD), tempName)
	published := false
	defer func() {
		_ = temp.Close()
		if !published {
			_ = unix.Unlinkat(shardFD, tempName, 0)
			_ = unix.Fsync(shardFD)
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
		n, err := temp.Write(chunk)
		if err != nil {
			return fmt.Errorf("write temporary object: %w", err)
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary object: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary object: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := regularEntryAt(shardFD, objectKey); err != nil {
		return err
	}
	if err := unix.Renameat(shardFD, tempName, shardFD, objectKey); err != nil {
		return fmt.Errorf("publish object: %w", err)
	}
	published = true
	if err := unix.Fsync(shardFD); err != nil {
		return fmt.Errorf("sync object directory: %w", err)
	}
	return nil
}

func (s *Filesystem) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	if err := validateObjectKey(objectKey); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	shardFD, closeDirs, err := s.openShard(objectKey, false)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, objectNotFound(objectKey)
		}
		return nil, err
	}
	defer closeDirs()

	fd, err := unix.Openat(shardFD, objectKey, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, objectNotFound(objectKey)
		}
		return nil, fmt.Errorf("open object: %w", err)
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("inspect object: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		unix.Close(fd)
		return nil, fmt.Errorf("object path is not a regular file")
	}
	if err := ctx.Err(); err != nil {
		unix.Close(fd)
		return nil, err
	}
	return os.NewFile(uintptr(fd), objectKey), nil
}

func (s *Filesystem) Delete(ctx context.Context, objectKey string) error {
	if err := validateObjectKey(objectKey); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	shardFD, closeDirs, err := s.openShard(objectKey, false)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer closeDirs()

	exists, err := regularEntryAt(shardFD, objectKey)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := unix.Unlinkat(shardFD, objectKey, 0); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return fmt.Errorf("delete object: %w", err)
	}
	if err := unix.Fsync(shardFD); err != nil {
		return fmt.Errorf("sync object directory: %w", err)
	}
	return nil
}

func (s *Filesystem) openRoot() (int, error) {
	fd, err := unix.Open(s.root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, fmt.Errorf("open filesystem object store root: %w", err)
	}
	return fd, nil
}

func (s *Filesystem) openShard(key string, create bool) (int, func(), error) {
	rootFD, err := s.openRoot()
	if err != nil {
		return -1, nil, err
	}
	fds := []int{rootFD}
	closeDirs := func() {
		for i := len(fds) - 1; i >= 0; i-- {
			_ = unix.Close(fds[i])
		}
	}

	parentFD := rootFD
	for _, name := range []string{"objects", key[:2], key[2:4]} {
		var fd int
		if create {
			fd, err = ensureDirAt(parentFD, name)
		} else {
			fd, err = openDirAt(parentFD, name)
		}
		if err != nil {
			closeDirs()
			return -1, nil, err
		}
		fds = append(fds, fd)
		parentFD = fd
	}
	return parentFD, closeDirs, nil
}

func ensureDirAt(parentFD int, name string) (int, error) {
	created := false
	if err := unix.Mkdirat(parentFD, name, 0o700); err != nil {
		if !errors.Is(err, unix.EEXIST) {
			return -1, fmt.Errorf("create object directory %q: %w", name, err)
		}
	} else {
		created = true
	}
	fd, err := openDirAt(parentFD, name)
	if err != nil {
		return -1, err
	}
	if err := unix.Fchmod(fd, 0o700); err != nil {
		unix.Close(fd)
		return -1, fmt.Errorf("set object directory %q permissions: %w", name, err)
	}
	if created {
		if err := unix.Fsync(parentFD); err != nil {
			unix.Close(fd)
			return -1, fmt.Errorf("sync parent of object directory %q: %w", name, err)
		}
	}
	return fd, nil
}

func openDirAt(parentFD int, name string) (int, error) {
	fd, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, fmt.Errorf("open object directory %q: %w", name, err)
	}
	return fd, nil
}

func regularEntryAt(parentFD int, name string) (bool, error) {
	var stat unix.Stat_t
	err := unix.Fstatat(parentFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect object path: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return false, fmt.Errorf("object path is not a regular file")
	}
	return true, nil
}

func createTempAt(parentFD int, prefix string) (string, int, error) {
	for range 100 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", -1, err
		}
		name := fmt.Sprintf("%s%x", prefix, random)
		fd, err := unix.Openat(parentFD, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return "", -1, err
		}
		if err := unix.Fchmod(fd, 0o600); err != nil {
			unix.Close(fd)
			_ = unix.Unlinkat(parentFD, name, 0)
			return "", -1, err
		}
		return name, fd, nil
	}
	return "", -1, fmt.Errorf("could not allocate a unique temporary file")
}
