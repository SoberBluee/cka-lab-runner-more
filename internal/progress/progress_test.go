package progress

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMarkCompleteAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "progress.yaml")

	s := empty()
	if s.IsComplete("lab_a") {
		t.Fatal("expected incomplete")
	}
	s.StartTimer("lab_a")
	time.Sleep(10 * time.Millisecond)
	dur := s.MarkComplete("lab_a")
	if dur < 0 {
		t.Fatal("expected non-negative duration")
	}
	if !s.IsComplete("lab_a") {
		t.Fatal("expected complete")
	}
	if err := Save(s, path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.IsComplete("lab_a") {
		t.Fatal("expected lab_a complete after reload")
	}
	if loaded.IsComplete("lab_b") {
		t.Fatal("expected lab_b incomplete")
	}
	complete, timeCol := loaded.Status("lab_a")
	if !complete || timeCol == "-" {
		t.Fatalf("expected completed status with time, got complete=%v time=%q", complete, timeCol)
	}
}

func TestLoadMissing(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Count() != 0 {
		t.Fatalf("expected empty store, got %d", s.Count())
	}
}

func TestClearAndIncomplete(t *testing.T) {
	s := empty()
	s.StartTimer("x")
	s.MarkComplete("x")
	s.StartTimer("y")
	s.MarkComplete("y")
	s.MarkIncomplete("x")
	if s.IsComplete("x") || !s.IsComplete("y") {
		t.Fatal("incomplete/clear logic failed")
	}
	s.Clear()
	if s.Count() != 0 {
		t.Fatal("expected clear")
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:              "0s",
		5 * time.Second: "5s",
		65 * time.Second: "1m5s",
		3661 * time.Second: "1h1m1s",
	}
	for in, want := range cases {
		if got := FormatDuration(in); got != want {
			t.Fatalf("FormatDuration(%v)=%q want %q", in, got, want)
		}
	}
}

func TestStatusInProgress(t *testing.T) {
	s := empty()
	s.StartTimer("running")
	complete, col := s.Status("running")
	if complete {
		t.Fatal("expected incomplete")
	}
	if col == "-" || col[len(col)-1] != '*' {
		t.Fatalf("expected running marker, got %q", col)
	}
}

func TestLegacyMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.yaml")
	raw := "completed:\n  old_lab: \"2020-01-01T00:00:00Z\"\n"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.IsComplete("old_lab") {
		t.Fatal("expected legacy completed lab migrated")
	}
}
