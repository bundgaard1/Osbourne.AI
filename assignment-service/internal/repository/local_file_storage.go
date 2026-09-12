package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidPath = errors.New("invalid storage path: path traversal detected")
	ErrNotFound    = errors.New("file not found")
)

type LocalFileStorage struct {
	baseDir string
}

func NewLocalFileStorage(baseDir string) (*LocalFileStorage, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute base path: %w", err)
	}

	if err := os.MkdirAll(absBase, 0750); err != nil {
		return nil, fmt.Errorf("failed to create upload base directory: %w", err)
	}
	return &LocalFileStorage{baseDir: absBase}, nil
}

// resolvePath guarantees that target paths remain strictly inside baseDir.
func (s *LocalFileStorage) resolvePath(rel string) (string, error) {
	// Clean the path to eliminate redundant '.' or '..' segments
	cleanRel := filepath.Clean(filepath.FromSlash(rel))
	if strings.HasPrefix(cleanRel, "..") || cleanRel == "." || cleanRel == "/" {
		return "", ErrInvalidPath
	}

	target := filepath.Join(s.baseDir, cleanRel)

	// Ensure the evaluated target has the baseDir prefix
	relCheck, err := filepath.Rel(s.baseDir, target)
	if err != nil || strings.HasPrefix(relCheck, "..") {
		return "", ErrInvalidPath
	}

	return target, nil
}

func (s *LocalFileStorage) Save(ctx context.Context, relativePath string, src io.Reader) (int64, error) {
	fullPath, err := s.resolvePath(relativePath)
	if err != nil {
		return 0, err
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return 0, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Create temp file in the same directory to ensure atomic os.Rename across filesystem boundaries
	tmpFile, err := os.CreateTemp(dir, ".upload-*.tmp")
	if err != nil {
		return 0, fmt.Errorf("failed to create temporary file: %w", err)
	}

	tmpPath := tmpFile.Name()
	// Clean up temp file on failure
	cleanup := true
	defer func() {
		tmpFile.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	// Copy data while honoring context cancellation
	written, err := copyWithContext(ctx, tmpFile, src)
	if err != nil {
		return 0, fmt.Errorf("failed to write contents: %w", err)
	}

	// Flush OS write buffers before rename
	if err := tmpFile.Sync(); err != nil {
		return 0, fmt.Errorf("failed to sync file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return 0, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename replaces any existing file or installs the new file in one operation
	if err := os.Rename(tmpPath, fullPath); err != nil {
		return 0, fmt.Errorf("failed to commit file: %w", err)
	}

	cleanup = false
	return written, nil
}

func (s *LocalFileStorage) Get(ctx context.Context, relativePath string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fullPath, err := s.resolvePath(relativePath)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

func (s *LocalFileStorage) Delete(ctx context.Context, relativePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	fullPath, err := s.resolvePath(relativePath)
	if err != nil {
		return err
	}

	err = os.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	return nil
}

// copyWithContext periodically checks context cancellation during stream operations.
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64

	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}

		nr, readErr := src.Read(buf)
		if nr > 0 {
			nw, writeErr := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if writeErr == nil {
					writeErr = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if writeErr != nil {
				return written, writeErr
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return written, readErr
		}
	}
	return written, nil
}
