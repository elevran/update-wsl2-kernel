package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
)

var (
	emptySHA1 = fmt.Sprintf("%x", sha1.New().Sum(nil))
)

// return the SHA1 digest for the named file
func sha1sum(fn string) (string, error) {
	if _, err := os.Stat(fn); err != nil {
		return emptySHA1, fmt.Errorf("failed to stat %s: %w", fn, err)
	}

	file, err := os.Open(fn)
	if err != nil {
		return emptySHA1, fmt.Errorf("failed to open %s: %w", fn, err)
	}
	defer file.Close()

	h := sha1.New()
	_, err = io.Copy(h, file)
	if err != nil {
		return emptySHA1, fmt.Errorf("failed to checksum %s: %w", fn, err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
