package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type CommandSpec struct {
	Name string
	Args []string
	Dir  string
}

type Runner interface {
	Run(ctx context.Context, spec CommandSpec, onStdout func(string)) error
}

type ExecRunner struct{}

func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

func (r *ExecRunner) Run(ctx context.Context, spec CommandSpec, onStdout func(string)) error {
	trimmedName := strings.TrimSpace(spec.Name)
	if trimmedName == "" {
		return fmt.Errorf("command name is required")
	}

	cmd := exec.CommandContext(ctx, trimmedName, spec.Args...)
	if strings.TrimSpace(spec.Dir) != "" {
		cmd.Dir = spec.Dir
	}

	writer := newStreamingLineWriter(onStdout)
	cmd.Stdout = writer
	cmd.Stderr = writer

	err := cmd.Run()
	writer.Flush()

	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if filepath.Base(trimmedName) == "rsync" {
			if mappedErr := MapRsyncExitCode(exitErr.ExitCode()); mappedErr != nil {
				return mappedErr
			}
		}
		return fmt.Errorf("command %q exited with code %d", trimmedName, exitErr.ExitCode())
	}

	return fmt.Errorf("run command %q: %w", trimmedName, err)
}

type streamingLineWriter struct {
	mu    sync.Mutex
	buf   []byte
	emit  func(string)
	flush func([]byte)
}

func newStreamingLineWriter(emit func(string)) *streamingLineWriter {
	writer := &streamingLineWriter{emit: emit}
	writer.flush = func(buffer []byte) {
		if writer.emit == nil || len(buffer) == 0 {
			return
		}
		line := strings.TrimSpace(string(buffer))
		if line != "" {
			writer.emit(line)
		}
	}
	return writer
}

func (w *streamingLineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, currentByte := range p {
		switch currentByte {
		case '\n', '\r':
			w.flush(w.buf)
			w.buf = w.buf[:0]
		default:
			w.buf = append(w.buf, currentByte)
		}
	}

	return len(p), nil
}

func (w *streamingLineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.flush(w.buf)
	w.buf = w.buf[:0]
}