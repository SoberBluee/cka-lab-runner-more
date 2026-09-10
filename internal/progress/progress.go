package progress

import (
	"fmt"
	"os"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultFile = "cka-lab-progress.yaml"

// LabRecord tracks attempt timing and completion for one lab.
type LabRecord struct {
	StartedAt   string `yaml:"started_at,omitempty"`
	CompletedAt string `yaml:"completed_at,omitempty"`
	DurationSec int64  `yaml:"duration_seconds,omitempty"`
	BestScore   int    `yaml:"best_score,omitempty"`
}

// Store tracks lab progress and timers.
type Store struct {
	// Completed is legacy (lab ID -> completion timestamp). Migrated into Labs on load.
	Completed map[string]string    `yaml:"completed,omitempty"`
	Labs      map[string]LabRecord `yaml:"labs"`
}

func empty() *Store {
	return &Store{
		Completed: map[string]string{},
		Labs:      map[string]LabRecord{},
	}
}

// Load reads progress from path. Missing file yields an empty store.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty(), nil
		}
		return nil, fmt.Errorf("reading progress file: %w", err)
	}

	var s Store
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing progress file: %w", err)
	}
	if s.Labs == nil {
		s.Labs = map[string]LabRecord{}
	}
	if s.Completed == nil {
		s.Completed = map[string]string{}
	}
	s.migrateLegacy()
	return &s, nil
}

func (s *Store) migrateLegacy() {
	for id, completedAt := range s.Completed {
		rec := s.Labs[id]
		if rec.CompletedAt == "" {
			rec.CompletedAt = completedAt
			s.Labs[id] = rec
		}
	}
}

// Save writes progress to path.
func Save(s *Store, path string) error {
	if s.Labs == nil {
		s.Labs = map[string]LabRecord{}
	}
	// Keep Completed in sync for older readers
	s.Completed = map[string]string{}
	for id, rec := range s.Labs {
		if rec.CompletedAt != "" {
			s.Completed[id] = rec.CompletedAt
		}
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshaling progress: %w", err)
	}
	header := "# cka-lab-runner progress — timers + labs marked complete after successful verify\n"
	if err := os.WriteFile(path, append([]byte(header), data...), 0644); err != nil {
		return fmt.Errorf("writing progress file: %w", err)
	}
	return nil
}

// IsComplete reports whether labID has been marked done.
func (s *Store) IsComplete(labID string) bool {
	if s == nil || s.Labs == nil {
		return false
	}
	rec, ok := s.Labs[labID]
	return ok && rec.CompletedAt != ""
}

// StartTimer records the start of a lab attempt (after the scenario is applied).
func (s *Store) StartTimer(labID string) {
	if s.Labs == nil {
		s.Labs = map[string]LabRecord{}
	}
	rec := s.Labs[labID]
	rec.StartedAt = time.Now().UTC().Format(time.RFC3339)
	rec.CompletedAt = ""
	rec.DurationSec = 0
	s.Labs[labID] = rec
}

// MarkComplete records a successful verify and stops the timer.
// Returns the attempt duration.
func (s *Store) MarkComplete(labID string) time.Duration {
	if s.Labs == nil {
		s.Labs = map[string]LabRecord{}
	}
	now := time.Now().UTC()
	rec := s.Labs[labID]
	rec.CompletedAt = now.Format(time.RFC3339)

	var dur time.Duration
	if rec.StartedAt != "" {
		if started, err := time.Parse(time.RFC3339, rec.StartedAt); err == nil {
			dur = now.Sub(started)
			if dur < 0 {
				dur = 0
			}
		}
	}
	rec.DurationSec = int64(dur.Seconds())
	s.Labs[labID] = rec
	return dur
}

func (s *Store) RecordScore(labID string, score int) int {
	if s.Labs == nil {
		s.Labs = map[string]LabRecord{}
	}
	rec := s.Labs[labID]
	if score > rec.BestScore {
		rec.BestScore = score
		s.Labs[labID] = rec
	}
	return rec.BestScore
}

func (s *Store) Score(labID string) int {
	if s == nil || s.Labs == nil {
		return 0
	}
	return s.Labs[labID].BestScore
}

// MarkIncomplete removes completion and timing for labID.
func (s *Store) MarkIncomplete(labID string) {
	if s.Labs == nil {
		return
	}
	delete(s.Labs, labID)
	if s.Completed != nil {
		delete(s.Completed, labID)
	}
}

// Clear removes all progress records.
func (s *Store) Clear() {
	s.Completed = map[string]string{}
	s.Labs = map[string]LabRecord{}
}

// CompletedIDs returns sorted completed lab IDs.
func (s *Store) CompletedIDs() []string {
	ids := make([]string, 0)
	for id, rec := range s.Labs {
		if rec.CompletedAt != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// Count returns how many labs are marked complete.
func (s *Store) Count() int {
	return len(s.CompletedIDs())
}

// Status returns completion and a display string for the Time column.
func (s *Store) Status(labID string) (complete bool, timeCol string) {
	if s == nil || s.Labs == nil {
		return false, "-"
	}
	rec, ok := s.Labs[labID]
	if !ok {
		return false, "-"
	}

	complete = rec.CompletedAt != ""
	if complete {
		if rec.DurationSec > 0 {
			return true, FormatDuration(time.Duration(rec.DurationSec) * time.Second)
		}
		if rec.StartedAt != "" {
			if started, err := time.Parse(time.RFC3339, rec.StartedAt); err == nil {
				if ended, err := time.Parse(time.RFC3339, rec.CompletedAt); err == nil {
					return true, FormatDuration(ended.Sub(started))
				}
			}
		}
		return true, "-"
	}
	if rec.StartedAt != "" {
		if started, err := time.Parse(time.RFC3339, rec.StartedAt); err == nil {
			elapsed := time.Since(started)
			if elapsed < 0 {
				elapsed = 0
			}
			return false, FormatDuration(elapsed) + "*"
		}
	}
	return false, "-"
}

// FormatDuration renders a short human duration like 1h2m3s / 4m5s / 12s.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh%dm%ds", h, m, sec)
	case m > 0:
		return fmt.Sprintf("%dm%ds", m, sec)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}
