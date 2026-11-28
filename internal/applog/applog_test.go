package applog

import (
	"os"
	"strings"
	"testing"
)

func TestBufferDisabledByDefaultRingCapClearExportACLOGS0102(t *testing.T) {
	buf := NewBuffer(3)
	if buf.Enabled() {
		t.Fatalf("expected buffer to be disabled by default")
	}

	buf.Logf("app", "should be ignored while disabled")
	if len(buf.Entries()) != 0 {
		t.Fatalf("expected 0 entries when disabled, got %d", len(buf.Entries()))
	}

	buf.SetEnabled(true)
	for _, msg := range []string{"m1", "m2", "m3", "m4"} {
		buf.Logf("core", "%s", msg)
	}
	entries := buf.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected ring buffer capped at 3, got %d", len(entries))
	}
	if entries[0].Message != "m2" || entries[2].Message != "m4" {
		t.Fatalf("unexpected ring buffer entries: %+v", entries)
	}

	exportPath, content, err := buf.Export(t.TempDir())
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if !strings.Contains(content, "[core] m4") {
		t.Fatalf("unexpected exported content: %q", content)
	}
	raw, err := os.ReadFile(exportPath)
	if err != nil || string(raw) != content {
		t.Fatalf("exported file mismatch: err=%v", err)
	}

	buf.Clear()
	if len(buf.Entries()) != 0 {
		t.Fatalf("expected 0 entries after Clear")
	}
}
