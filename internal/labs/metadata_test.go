package labs

import (
	"strings"
	"testing"
)

func TestEveryLabHasUsableMetadata(t *testing.T) {
	validCategories := map[Category]bool{
		CategoryControlPlane: true,
		CategoryNetworking:   true,
		CategoryScheduling:   true,
		CategoryDNS:          true,
		CategoryStorage:      true,
		CategoryWorkloads:    true,
		CategoryRBAC:         true,
		CategorySecurity:     true,
		CategoryExam:         true,
	}
	validDifficulties := map[Difficulty]bool{
		DifficultyEasy:   true,
		DifficultyMedium: true,
		DifficultyHard:   true,
	}

	for _, lab := range List() {
		id := lab.ID()
		if lab.Title() == "" {
			t.Errorf("%s: empty title", id)
		}
		if lab.Description() == "" {
			t.Errorf("%s: empty description", id)
		}
		if !validCategories[lab.Category()] {
			t.Errorf("%s: unknown category %q", id, lab.Category())
		}
		if !validDifficulties[lab.Difficulty()] {
			t.Errorf("%s: unknown difficulty %q", id, lab.Difficulty())
		}
		if lab.EstimatedTime() <= 0 {
			t.Errorf("%s: estimated time must be positive", id)
		}
		// Exam-style labs (cka-lab-author skill) must not ship hints.
		desc := lab.Description()
		if strings.Contains(desc, "[Weight:") {
			if len(lab.Hints()) != 0 {
				t.Errorf("%s: exam-style lab must not include hints", id)
			}
			if !strings.Contains(desc, "kubectl config use-context") {
				t.Errorf("%s: exam-style lab must include kubectl config use-context", id)
			}
			if !strings.Contains(desc, "Time limit:") {
				t.Errorf("%s: exam-style lab must include a time limit", id)
			}
		} else if len(lab.Hints()) == 0 {
			t.Errorf("%s: no hints", id)
		}
		if len(lab.SolutionSteps()) == 0 {
			t.Errorf("%s: no solution steps", id)
		}
		for i, step := range lab.SolutionSteps() {
			if step.Description == "" {
				t.Errorf("%s: solution step %d has no description", id, i+1)
			}
		}
	}
}
