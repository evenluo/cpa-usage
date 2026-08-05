package logging

import (
	"bytes"
	"context"
	stdlog "log"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"github.com/gin-gonic/gin"
)

func TestParseSlogLevelMapsConfiguredLevels(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  slog.Level
	}{
		{name: "trace maps to debug", input: "trace", want: slog.LevelDebug},
		{name: "debug", input: "debug", want: slog.LevelDebug},
		{name: "warn", input: "warn", want: slog.LevelWarn},
		{name: "warning alias", input: "WARNING", want: slog.LevelWarn},
		{name: "error", input: "error", want: slog.LevelError},
		{name: "fatal collapses to error", input: "fatal", want: slog.LevelError},
		{name: "panic collapses to error", input: "panic", want: slog.LevelError},
		{name: "unknown defaults to info", input: "verbose", want: slog.LevelInfo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSlogLevel(tc.input); got != tc.want {
				t.Fatalf("parseSlogLevel(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolveLogDirUsesWorkDirFallback(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "work")

	logDir := resolveLogDir(config.Config{WorkDir: workDir})

	if logDir != filepath.Join(workDir, filepath.Base(config.DefaultLogDir)) {
		t.Fatalf("expected log dir under work dir, got %q", logDir)
	}
}

func TestConfigureWritesSlogToDailyFile(t *testing.T) {
	reset := captureGlobalLogState(t)
	defer reset()

	logDir := t.TempDir()
	closer, err := Configure(config.Config{
		LogLevel:         "info",
		LogFileEnabled:   true,
		LogDir:           logDir,
		LogRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	defer closer.Close()

	slog.Info("file logging works")

	content := readTodayLogFile(t, logDir)
	if !strings.Contains(content, "file logging works") {
		t.Fatalf("expected log file to contain slog message, got %q", content)
	}
	if !logLineHasTimestamp(content) {
		t.Fatalf("expected log file to include timestamp, got %q", content)
	}
}

func TestConfigureUsesTextHandler(t *testing.T) {
	reset := captureGlobalLogState(t)
	defer reset()

	closer, err := Configure(config.Config{
		LogLevel:         "info",
		LogFileEnabled:   false,
		LogRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	defer closer.Close()

	if _, ok := slog.Default().Handler().(*slog.TextHandler); !ok {
		t.Fatalf("expected text handler, got %T", slog.Default().Handler())
	}
}

func TestConfigureWritesSlogConsoleWithTimestamp(t *testing.T) {
	reset := captureGlobalLogState(t)
	defer reset()

	previousStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = previousStderr
		_ = reader.Close()
	}()

	closer, err := Configure(config.Config{
		LogLevel:         "info",
		LogFileEnabled:   false,
		LogRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}

	slog.Info("console timestamp works")
	if err := closer.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	var output bytes.Buffer
	if _, err := output.ReadFrom(reader); err != nil {
		t.Fatalf("read stderr output: %v", err)
	}
	content := output.String()
	if !strings.Contains(content, "console timestamp works") {
		t.Fatalf("expected console output to contain slog message, got %q", content)
	}
	if !logLineHasTimestamp(content) {
		t.Fatalf("expected console output to include timestamp, got %q", content)
	}
}

func TestConfigureDisablesFileLogging(t *testing.T) {
	reset := captureGlobalLogState(t)
	defer reset()

	logDir := t.TempDir()
	closer, err := Configure(config.Config{
		LogLevel:         "info",
		LogFileEnabled:   false,
		LogDir:           logDir,
		LogRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	defer closer.Close()

	slog.Info("stderr only")

	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("read log dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no log files when file logging disabled, got %d", len(entries))
	}
}

func TestConfigureRoutesStdlibLogAndSlogToFile(t *testing.T) {
	reset := captureGlobalLogState(t)
	defer reset()

	logDir := t.TempDir()
	closer, err := Configure(config.Config{
		LogLevel:         "info",
		LogFileEnabled:   true,
		LogDir:           logDir,
		LogRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	defer closer.Close()

	stdlog.Print("stdlib message")
	slog.Error("slog message")

	content := readTodayLogFile(t, logDir)
	if !strings.Contains(content, "stdlib message") || !strings.Contains(content, "slog message") {
		t.Fatalf("expected stdlib and slog messages in file, got %q", content)
	}
}

func TestConfigureRoutesGinDebugToTimestampedSlogOutput(t *testing.T) {
	reset := captureGlobalLogState(t)
	defer reset()

	previousStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = previousStderr
		_ = reader.Close()
	}()

	closer, err := Configure(config.Config{
		LogLevel:         "info",
		LogFileEnabled:   false,
		LogRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}

	if gin.DebugPrintFunc == nil {
		t.Fatal("expected Configure to install Gin debug print function")
	}
	gin.DebugPrintFunc("GET %s", "/api/v1/status")
	if err := closer.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	var output bytes.Buffer
	if _, err := output.ReadFrom(reader); err != nil {
		t.Fatalf("read stderr output: %v", err)
	}
	content := output.String()
	if !strings.Contains(content, "[GIN-debug] GET /api/v1/status") {
		t.Fatalf("expected Gin debug output to be routed through slog, got %q", content)
	}
	if !logLineHasTimestamp(content) {
		t.Fatalf("expected Gin debug output to include timestamp, got %q", content)
	}
}

func TestConfigureCloseRestoresGlobalLoggers(t *testing.T) {
	reset := captureGlobalLogState(t)
	defer reset()

	var restoredOutput bytes.Buffer
	stdlog.SetOutput(&restoredOutput)
	slog.SetDefault(slog.New(slog.NewTextHandler(&restoredOutput, nil)))
	gin.DefaultWriter = &restoredOutput
	gin.DefaultErrorWriter = &restoredOutput
	gin.DebugPrintFunc = func(format string, values ...interface{}) {
		restoredOutput.WriteString("after close gin debug")
	}
	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		restoredOutput.WriteString(" after close gin route")
	}

	closer, err := Configure(config.Config{
		LogLevel:         "info",
		LogFileEnabled:   true,
		LogDir:           t.TempDir(),
		LogRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	stdlog.Print("after close stdlib")
	slog.Error("after close slog")
	gin.DebugPrintFunc("ignored")
	gin.DebugPrintRouteFunc("GET", "/", "handler", 1)

	content := restoredOutput.String()
	for _, want := range []string{"after close stdlib", "after close slog", "after close gin debug", "after close gin route"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected global loggers to be restored after close with %q, got %q", want, content)
		}
	}
}

func TestConfigureErrorLeavesGlobalLoggerStateUnchanged(t *testing.T) {
	reset := captureGlobalLogState(t)
	defer reset()

	previousLevel := slog.LevelDebug
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: previousLevel})))
	invalidLogDir := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(invalidLogDir, []byte("file"), 0644); err != nil {
		t.Fatalf("write invalid log dir fixture: %v", err)
	}

	_, err := Configure(config.Config{
		LogLevel:         "error",
		LogFileEnabled:   true,
		LogDir:           invalidLogDir,
		LogRetentionDays: 7,
	})
	if err == nil {
		t.Fatal("expected Configure to return an error")
	}
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("expected slog level to remain debug after configure error")
	}
}

func TestRetentionDeletesOnlyOldAppLogs(t *testing.T) {
	logDir := t.TempDir()
	oldAppLog := filepath.Join(logDir, "cpa-usage-2020-01-01.log")
	freshAppLog := filepath.Join(logDir, "cpa-usage-2099-01-01.log")
	otherLog := filepath.Join(logDir, "other.log")
	for _, path := range []string{oldAppLog, freshAppLog, otherLog} {
		if err := os.WriteFile(path, []byte("log"), 0644); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	writer, err := newDailyFileWriter(logDir, 7, func() time.Time {
		return time.Date(2026, 4, 28, 12, 0, 0, 0, time.Local)
	})
	if err != nil {
		t.Fatalf("newDailyFileWriter returned error: %v", err)
	}
	defer writer.Close()

	if _, err := os.Stat(oldAppLog); !os.IsNotExist(err) {
		t.Fatalf("expected old app log to be removed, stat err=%v", err)
	}
	for _, path := range []string{freshAppLog, otherLog} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to remain: %v", path, err)
		}
	}
}

func readTodayLogFile(t *testing.T, logDir string) string {
	t.Helper()
	path := filepath.Join(logDir, "cpa-usage-"+time.Now().Format("2006-01-02")+".log")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read today log file: %v", err)
	}
	return string(content)
}

func logLineHasTimestamp(content string) bool {
	return regexp.MustCompile(`time="?\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`).MatchString(content)
}

func captureGlobalLogState(t *testing.T) func() {
	t.Helper()
	previousStdlogOutput := stdlog.Writer()
	previousSlog := slog.Default()
	previousGinDefaultWriter := gin.DefaultWriter
	previousGinErrorWriter := gin.DefaultErrorWriter
	previousGinDebugPrint := gin.DebugPrintFunc
	previousGinDebugPrintRoute := gin.DebugPrintRouteFunc
	var stderr bytes.Buffer
	stdlog.SetOutput(&stderr)
	return func() {
		stdlog.SetOutput(previousStdlogOutput)
		slog.SetDefault(previousSlog)
		gin.DefaultWriter = previousGinDefaultWriter
		gin.DefaultErrorWriter = previousGinErrorWriter
		gin.DebugPrintFunc = previousGinDebugPrint
		gin.DebugPrintRouteFunc = previousGinDebugPrintRoute
	}
}
