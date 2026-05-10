package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logger *Logger
	once   sync.Once
)

type Logger struct {
	file *os.File
	mu   sync.Mutex
}

func Init() error {
	var initErr error
	once.Do(func() {
		var err error
		logger, err = newLogger()
		if err != nil {
			initErr = err
			return
		}
		logger.Printf("--- rdbatch started ---")
	})
	return initErr
}

func newLogger() (*Logger, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".local", "share", "rdbatch")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "rdbatch.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f}, nil
}

func (l *Logger) Printf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.file, "[%s] %s\n", timestamp, line)
	_ = l.file.Sync()
}

func Printf(format string, args ...interface{}) {
	if logger != nil {
		logger.Printf(format, args...)
	}
}
