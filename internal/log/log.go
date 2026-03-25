// internal/log/log.go
package log

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type contextKey struct{}

// Logger writes structured JSON log lines to stderr when enabled.
type Logger struct {
	enabled bool
	traceID string
}

// New creates a Logger. If enabled is false, all methods are no-ops.
func New(enabled bool) *Logger {
	return &Logger{enabled: enabled}
}

// WithTraceID returns a new Logger with a freshly generated trace_id.
func (l *Logger) WithTraceID() *Logger {
	if !l.enabled {
		return l
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return &Logger{enabled: true, traceID: fmt.Sprintf("%x", b)}
}

// Info logs an info-level event.
func (l *Logger) Info(step string, fields map[string]any) {
	l.write("info", step, fields)
}

// Error logs an error-level event.
func (l *Logger) Error(step string, fields map[string]any) {
	l.write("error", step, fields)
}

func (l *Logger) write(level, step string, fields map[string]any) {
	if !l.enabled {
		return
	}
	entry := map[string]any{
		"ts":       time.Now().UTC().Format(time.RFC3339),
		"level":    level,
		"step":     step,
		"trace_id": l.traceID,
	}
	for k, v := range fields {
		entry[k] = v
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "%s\n", data)
}

// WithLogger stores a Logger in the context.
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the Logger from context.
// Returns a no-op logger if none is present.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(contextKey{}).(*Logger); ok {
		return l
	}
	return &Logger{enabled: false}
}
