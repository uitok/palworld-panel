package server

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type rollingLogWriter struct {
	mu      sync.Mutex
	path    string
	maxSize int64
	backups int
	file    *os.File
	size    int64
}

func newRollingLogWriter(path string, maxSize int64, backups int) (*rollingLogWriter, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("log max size must be positive")
	}
	if backups < 1 {
		return nil, fmt.Errorf("log backup count must be positive")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	w := &rollingLogWriter{path: path, maxSize: maxSize, backups: backups}
	if err := w.open(); err != nil {
		return nil, err
	}
	if w.size >= w.maxSize {
		if err := w.rotate(); err != nil {
			_ = w.file.Close()
			return nil, err
		}
	}
	return w, nil
}

func (w *rollingLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	written := 0
	for len(p) > 0 {
		if w.size >= w.maxSize {
			if err := w.rotate(); err != nil {
				return written, err
			}
		}
		remaining := w.maxSize - w.size
		chunk := p
		if int64(len(chunk)) > remaining {
			chunk = p[:int(remaining)]
		}
		n, err := w.file.Write(chunk)
		written += n
		w.size += int64(n)
		p = p[n:]
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, fmt.Errorf("log writer made no progress")
		}
	}
	return written, nil
}

func (w *rollingLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *rollingLogWriter) open() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *rollingLogWriter) rotate() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.backups))
	for index := w.backups - 1; index >= 1; index-- {
		oldPath := fmt.Sprintf("%s.%d", w.path, index)
		newPath := fmt.Sprintf("%s.%d", w.path, index+1)
		if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.Rename(w.path, w.path+".1"); err != nil && !os.IsNotExist(err) {
		return err
	}
	return w.open()
}
