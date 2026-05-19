package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	StateDirName  = "luks-automount"
	LogFileName   = "luks-automount.log"
	MaxSizeMB     = 10
	MaxBackups    = 5
	MaxAgeDays    = 30
	Compress      = true
	DirPerm       = 0o700
)

func DefaultLogPath() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, LogFileName), nil
}

func StateDir() (string, error) {
	return stateDir()
}

func stateDir() (string, error) {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, StateDirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", StateDirName), nil
}

func Setup(level slog.Level) error {
	logPath, err := DefaultLogPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), DirPerm); err != nil {
		return err
	}

	rotator := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    MaxSizeMB,
		MaxBackups: MaxBackups,
		MaxAge:     MaxAgeDays,
		Compress:   Compress,
	}

	opts := &slog.HandlerOptions{Level: level}
	textHandler := slog.NewTextHandler(os.Stderr, opts)
	jsonHandler := slog.NewJSONHandler(rotator, opts)

	slog.SetDefault(slog.New(newFanoutHandler(textHandler, jsonHandler)))
	return nil
}

func SetupStderrOnly(level slog.Level) {
	opts := &slog.HandlerOptions{Level: level}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
}

func DiscardWriter() io.Writer {
	return io.Discard
}
