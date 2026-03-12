package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/abhishek/pen-drive/backend/internal/config"
)

func New(cfg config.LoggingConfig) (*slog.Logger, func() error, error) {
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, nil, err
	}

	rotatingFile := &lumberjack.Logger{
		Filename:   filepath.Clean(cfg.FilePath()),
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     14,
		Compress:   false,
	}

	writer := io.MultiWriter(os.Stdout, rotatingFile)
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})

	return slog.New(handler), rotatingFile.Close, nil
}
