package logging

import (
	"context"
	"fmt"
	"io"
	stdlog "log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cpa-usage/internal/config"
	"github.com/gin-gonic/gin"
)

const logFilePrefix = "cpa-usage-"

type noopCloser struct{}

func (noopCloser) Close() error { return nil }

type restoreCloser struct {
	closer                     io.Closer
	previousStdlogOutput       io.Writer
	previousSlog               *slog.Logger
	previousGinDefaultWriter   io.Writer
	previousGinErrorWriter     io.Writer
	previousGinDebugPrint      func(string, ...interface{})
	previousGinDebugPrintRoute func(string, string, string, int)
}

func (c *restoreCloser) Close() error {
	stdlog.SetOutput(c.previousStdlogOutput)
	slog.SetDefault(c.previousSlog)
	gin.DefaultWriter = c.previousGinDefaultWriter
	gin.DefaultErrorWriter = c.previousGinErrorWriter
	gin.DebugPrintFunc = c.previousGinDebugPrint
	gin.DebugPrintRouteFunc = c.previousGinDebugPrintRoute
	return c.closer.Close()
}

func resolveLogDir(cfg config.Config) string {
	logDir := strings.TrimSpace(cfg.LogDir)
	if logDir != "" {
		return logDir
	}
	workDir := strings.TrimSpace(cfg.WorkDir)
	if workDir == "" {
		workDir = config.DefaultWorkDir
	}
	return filepath.Join(workDir, filepath.Base(config.DefaultLogDir))
}

// parseSlogLevel 把 LOG_LEVEL 配置映射为 slog 级别。slog 没有低于 debug 的级别，
// 因此 logrus 的 trace 映射到 debug；fatal/panic 与 error 归一到 error。
func parseSlogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "trace":
		return slog.LevelDebug
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "fatal", "panic":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Configure(cfg config.Config) (io.Closer, error) {
	previousStdlogOutput := stdlog.Writer()
	previousSlog := slog.Default()
	previousGinDefaultWriter := gin.DefaultWriter
	previousGinErrorWriter := gin.DefaultErrorWriter
	previousGinDebugPrint := gin.DebugPrintFunc
	previousGinDebugPrintRoute := gin.DebugPrintRouteFunc

	level := parseSlogLevel(cfg.LogLevel)

	writer := io.Writer(os.Stderr)
	var closer io.Closer = noopCloser{}
	if cfg.LogFileEnabled {
		logDir := resolveLogDir(cfg)
		dailyWriter, err := newDailyFileWriter(logDir, cfg.LogRetentionDays, time.Now)
		if err != nil {
			return nil, err
		}
		writer = io.MultiWriter(os.Stderr, dailyWriter)
		closer = dailyWriter
	}

	stdlog.SetOutput(writer)
	slog.SetDefault(slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level})))
	configureGinLogging()
	return &restoreCloser{
		closer:                     closer,
		previousStdlogOutput:       previousStdlogOutput,
		previousSlog:               previousSlog,
		previousGinDefaultWriter:   previousGinDefaultWriter,
		previousGinErrorWriter:     previousGinErrorWriter,
		previousGinDebugPrint:      previousGinDebugPrint,
		previousGinDebugPrintRoute: previousGinDebugPrintRoute,
	}, nil
}

type slogWriter struct {
	level slog.Level
}

func (w slogWriter) Write(p []byte) (int, error) {
	message := strings.TrimRight(string(p), "\r\n")
	if message != "" {
		slog.Log(context.Background(), w.level, message)
	}
	return len(p), nil
}

func configureGinLogging() {
	gin.DefaultWriter = slogWriter{level: slog.LevelInfo}
	gin.DefaultErrorWriter = slogWriter{level: slog.LevelError}
	gin.DebugPrintFunc = func(format string, values ...interface{}) {
		message := fmt.Sprintf("[GIN-debug] "+strings.TrimRight(format, "\r\n"), values...)
		slog.Info(message)
	}
	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		slog.Info(fmt.Sprintf("[GIN-debug] %-6s %s --> %s (%d handlers)", httpMethod, absolutePath, handlerName, nuHandlers))
	}
}

type dailyFileWriter struct {
	mu            sync.Mutex
	dir           string
	retentionDays int
	now           func() time.Time
	currentDate   string
	file          *os.File
}

func newDailyFileWriter(dir string, retentionDays int, now func() time.Time) (*dailyFileWriter, error) {
	if now == nil {
		now = time.Now
	}
	writer := &dailyFileWriter{
		dir:           dir,
		retentionDays: retentionDays,
		now:           now,
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	if err := writer.rotateLocked(); err != nil {
		return nil, err
	}
	if err := writer.cleanupLocked(); err != nil {
		_ = writer.Close()
		return nil, err
	}
	return writer, nil
}

func (w *dailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	date := w.now().Format("2006-01-02")
	if w.file == nil || w.currentDate != date {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
		if err := w.cleanupLocked(); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *dailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *dailyFileWriter) rotateLocked() error {
	date := w.now().Format("2006-01-02")
	if w.file != nil && w.currentDate == date {
		return nil
	}
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("close log file: %w", err)
		}
	}
	filePath := filepath.Join(w.dir, logFilePrefix+date+".log")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	w.file = file
	w.currentDate = date
	return nil
}

func (w *dailyFileWriter) cleanupLocked() error {
	if w.retentionDays <= 0 {
		return nil
	}
	cutoff := w.now().AddDate(0, 0, -w.retentionDays)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return fmt.Errorf("read log dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, logFilePrefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		datePart := strings.TrimSuffix(strings.TrimPrefix(name, logFilePrefix), ".log")
		logDate, err := time.ParseInLocation("2006-01-02", datePart, time.Local)
		if err != nil {
			continue
		}
		if logDate.Before(dateOnly(cutoff)) {
			if err := os.Remove(filepath.Join(w.dir, name)); err != nil {
				return fmt.Errorf("remove old log file: %w", err)
			}
		}
	}
	return nil
}

func dateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}
