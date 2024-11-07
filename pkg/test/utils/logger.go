package utils

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/log"
)

// Logger is an interface used by the KUTTL test operator to provide logging of tests.
type Logger interface {
	Log(message string)
	LogWithArgs(message string, args ...interface{})
	Error(message string)
	ErrorWithArgs(message string, args ...interface{})
	WithNewBuffer() Logger
	WithGroup(string) Logger
	Write(p []byte) (n int, err error)
	Flush()
}

// TestLogger implements the Logger interface to be compatible with the go test operator's
// output buffering (without this, the use of Parallel tests combined with subtests causes test
// output to be mixed).
type TestLogger struct {
	test *testing.T
	// NOTE (@NickLarsenNZ): This buffer is for the Write impl, output from commands run. It is not the general log buffer.
	buffer []byte
	logger *slog.Logger
	// NOTE (@NickLarsenNZ): This will default to io.Stdout
	log_output io.Writer
}

// NewTestLogger creates a new test logger.
// NOTE (@NickLarsenNZ): A new logger can be made for general code, but then we should be able to make a new one with a new buffer.
// Eg:
//
//	kuttl_logger := NewTestLogger(...)
//	specific_case_logger := kuttl_logger.WithNewBuffer().WithGroup("case-1").WithGroup("step-1")
//
// Then we need to think about how to _get_ the logs from the buffer on failure, at the end.
//
//	specific_case_logger.Write(os.Stdout)
func NewTestLogger(test *testing.T, log_group string) *TestLogger {
	// TODO (@NickLarsenNZ): toggle stdout vs buffer
	// The complication is the layers of loggers (WithPrefix -> WithGroup) which would make the buffers disjoint.
	// So when the relevant buffers are read, logs are not interleaved anymore.

	handler := log.NewWithOptions(os.Stdout, log.Options{
		TimeFormat:      time.RFC3339, // Maybe want to use TimeOnly when run from an interactive terminal
		ReportTimestamp: true,
	})

	// TODO (@NickLarsenNZ): Remove WithGroup here, it can be done as the logger is passed down from the haress down to the steps
	logger := slog.New(handler).WithGroup(log_group)

	return &TestLogger{
		test:       test,
		buffer:     []byte{},
		logger:     logger,
		log_output: os.Stdout,
	}
}

// Log logs the provided arguments with the logger's prefix. See testing.Log for more details.
func (t *TestLogger) Log(message string) {
	t.logger.Info(message)
}

func (t *TestLogger) LogWithArgs(message string, args ...interface{}) {
	t.logger.Info(message, args...)
}

func (t *TestLogger) Error(message string) {
	t.logger.Error(message)
}

func (t *TestLogger) ErrorWithArgs(message string, args ...interface{}) {
	t.logger.Error(message, args...)
}

// NOTE (@NickLarsenNZ): This will copy the logger, but create a new buffer
func (t *TestLogger) WithNewBuffer() Logger {
	new_logger := t
	new_logger.log_output = new(bytes.Buffer)

	return new_logger
}

func (t *TestLogger) WithGroup(group string) Logger {
	new_logger := t
	new_logger.logger.WithGroup(group)

	return new_logger
}

// Write implements the io.Writer interface.
// Logs each line written to it, buffers incomplete lines until the next Write() call.
// NOTE (@NickLarsenNZ): I believe this is needed so the logger can be passed to the command/script executer.
func (t *TestLogger) Write(p []byte) (n int, err error) {
	t.buffer = append(t.buffer, p...)

	splitBuf := bytes.Split(t.buffer, []byte{'\n'})
	t.buffer = splitBuf[len(splitBuf)-1]

	for _, line := range splitBuf[:len(splitBuf)-1] {
		t.Log(string(line))
	}

	return len(p), nil
}

func (t *TestLogger) Flush() {
	if len(t.buffer) != 0 {
		t.Log(string(t.buffer))
		t.buffer = []byte{}
	}
}
