package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultCapacity = 2000

type Entry struct {
	Time    string `json:"time"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

// Buffer is a thread-safe bounded ring buffer for runtime diagnostic logs.
// It starts disabled by default and records nothing until SetEnabled(true).
type Buffer struct {
	mu      sync.Mutex
	enabled atomic.Bool
	cap     int
	entries []Entry
}

func NewBuffer(capacity int) *Buffer {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Buffer{
		cap:     capacity,
		entries: make([]Entry, 0, min(capacity, 64)),
	}
}

func (b *Buffer) SetEnabled(enabled bool) {
	b.enabled.Store(enabled)
}

func (b *Buffer) Enabled() bool {
	return b.enabled.Load()
}

func (b *Buffer) Append(source, message string) {
	if !b.enabled.Load() {
		return
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return
	}
	if source == "" {
		source = "app"
	}
	entry := Entry{
		Time:    time.Now().Format("15:04:05.000"),
		Source:  source,
		Message: msg,
	}
	b.mu.Lock()
	if len(b.entries) >= b.cap {
		copy(b.entries, b.entries[1:])
		b.entries[len(b.entries)-1] = entry
	} else {
		b.entries = append(b.entries, entry)
	}
	b.mu.Unlock()
}

func (b *Buffer) Logf(source, format string, args ...any) {
	if !b.enabled.Load() {
		return
	}
	b.Append(source, fmt.Sprintf(format, args...))
}

// Write implements io.Writer so Buffer can capture Go standard logger output.
func (b *Buffer) Write(p []byte) (int, error) {
	if b.enabled.Load() {
		for _, line := range strings.Split(string(p), "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				b.Append("go", trimmed)
			}
		}
	}
	return len(p), nil
}

func (b *Buffer) Entries() []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Entry, len(b.entries))
	copy(out, b.entries)
	return out
}

func (b *Buffer) Text() string {
	entries := b.Entries()
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString("[")
		sb.WriteString(e.Time)
		sb.WriteString("] [")
		sb.WriteString(e.Source)
		sb.WriteString("] ")
		sb.WriteString(e.Message)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func (b *Buffer) Clear() {
	b.mu.Lock()
	b.entries = b.entries[:0]
	b.mu.Unlock()
}

func (b *Buffer) Export(exportDir string) (string, string, error) {
	if err := os.MkdirAll(exportDir, 0o700); err != nil {
		return "", "", err
	}
	content := b.Text()
	name := fmt.Sprintf("looma-logs-%s.log", time.Now().Format("20060102-150405"))
	path := filepath.Join(exportDir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", "", err
	}
	return path, content, nil
}
