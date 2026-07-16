package debuglog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultMaxBytes = int64(20 * 1024 * 1024)
	defaultMaxFiles = 5
)

// Logger is a runtime-switchable, bounded debug log. Callers must not write
// credentials, authorization headers, or request bodies to it.
type Logger struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	enabled  atomic.Bool
	maxBytes int64
	maxFiles int
}

type Status struct {
	Enabled  bool   `json:"enabled"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	MaxBytes int64  `json:"max_bytes"`
	MaxFiles int    `json:"max_files"`
}

func New(path string, enabled bool) (*Logger, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	logger := &Logger{path: filepath.Clean(abs), maxBytes: defaultMaxBytes, maxFiles: defaultMaxFiles}
	if enabled {
		if err := logger.SetEnabled(true); err != nil {
			return nil, err
		}
	}
	return logger, nil
}

func (l *Logger) Enabled() bool {
	return l != nil && l.enabled.Load()
}

func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *Logger) Status() Status {
	if l == nil {
		return Status{}
	}
	status := Status{Enabled: l.Enabled(), Path: l.path, MaxBytes: l.maxBytes, MaxFiles: l.maxFiles}
	if info, err := os.Stat(l.path); err == nil {
		status.Size = info.Size()
	}
	return status
}

func (l *Logger) SetEnabled(enabled bool) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if enabled {
		if err := l.ensureFileLocked(); err != nil {
			return err
		}
		l.enabled.Store(true)
		_, err := l.writeLocked([]byte(timestamped("debug logging enabled")))
		return err
	}
	if l.enabled.Load() && l.file != nil {
		_, _ = l.writeLocked([]byte(timestamped("debug logging disabled")))
	}
	l.enabled.Store(false)
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

func (l *Logger) Printf(format string, args ...any) {
	if !l.Enabled() {
		return
	}
	_, _ = io.WriteString(l, timestamped(fmt.Sprintf(format, args...)))
}

// Write lets the standard library logger mirror its normal output into the
// debug file while stderr or journald remains the primary destination.
func (l *Logger) Write(payload []byte) (int, error) {
	if l == nil || !l.Enabled() {
		return len(payload), nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.writeLocked(payload)
}

func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled.Store(false)
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func (l *Logger) writeLocked(payload []byte) (int, error) {
	if err := l.ensureFileLocked(); err != nil {
		return 0, err
	}
	if err := l.rotateLocked(int64(len(payload))); err != nil {
		return 0, err
	}
	return l.file.Write(payload)
}

func (l *Logger) ensureFileLocked() error {
	if l.file != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	l.file = file
	return nil
}

func (l *Logger) rotateLocked(incoming int64) error {
	info, err := l.file.Stat()
	if err != nil || info.Size()+incoming <= l.maxBytes {
		return err
	}
	if err := l.file.Close(); err != nil {
		return err
	}
	l.file = nil
	for index := l.maxFiles; index >= 1; index-- {
		source := l.path
		if index > 1 {
			source = fmt.Sprintf("%s.%d", l.path, index-1)
		}
		destination := fmt.Sprintf("%s.%d", l.path, index)
		_ = os.Remove(destination)
		if err := os.Rename(source, destination); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return l.ensureFileLocked()
}

func timestamped(message string) string {
	return time.Now().UTC().Format(time.RFC3339Nano) + " DEBUG " + message + "\n"
}
