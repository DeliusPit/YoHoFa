package objectstore

import (
	"errors"
	"fmt"
	"io/fs"
)

var (
	ErrInvalidKey = errors.New("invalid object key")
	ErrNotFound   = fs.ErrNotExist
)

func validateObjectKey(key string) error {
	if len(key) != 64 {
		return fmt.Errorf("%w: key must contain 64 lowercase hexadecimal characters", ErrInvalidKey)
	}
	for i := range key {
		if (key[i] < '0' || key[i] > '9') && (key[i] < 'a' || key[i] > 'f') {
			return fmt.Errorf("%w: key must contain 64 lowercase hexadecimal characters", ErrInvalidKey)
		}
	}
	return nil
}

func objectNotFound(key string) error {
	return fmt.Errorf("object %s: %w", key, ErrNotFound)
}
