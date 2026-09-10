package cli

import (
	"fmt"
	"strings"

	"github.com/CuriousLearner/cka-lab-runner/internal/labs"
	"github.com/CuriousLearner/cka-lab-runner/internal/progress"
)

// PrintLabList prints a formatted list of labs with completion and timer columns.
func PrintLabList(labList []labs.Lab, store *progress.Store) {
	if len(labList) == 0 {
		fmt.Println("No labs available.")
		return
	}
	if store == nil {
		store = &progress.Store{}
	}

	fmt.Printf("%-6s %-10s %-9s %-28s %-36s %-16s %-10s\n", "Done", "Time", "Best", "ID", "Title", "Category", "Difficulty")
	fmt.Println(strings.Repeat("─", 122))

	doneCount := 0
	for _, lab := range labList {
		info := labs.GetInfo(lab)
		complete, timeCol := store.Status(info.ID)
		mark := "✗"
		if complete {
			mark = "✓"
			doneCount++
		}
		scoreCol := "-"
		if _, ok := lab.(labs.ScoredLab); ok {
			scoreCol = fmt.Sprintf("%d/100", store.Score(info.ID))
		}
		fmt.Printf("%-6s %-10s %-9s %-28s %-36s %-16s %-10s\n",
			mark,
			truncate(timeCol, 10),
			scoreCol,
			info.ID,
			truncate(info.Title, 34),
			info.Category,
			info.Difficulty,
		)
	}
	fmt.Println(strings.Repeat("─", 122))
	fmt.Printf("Progress: %d/%d completed  (* = timer running)\n", doneCount, len(labList))
}

func PrintExamReport(report labs.ExamReport) {
	for _, task := range report.Tasks {
		mark := "✗"
		if task.Score() == task.Weight {
			mark = "✓"
		}
		fmt.Printf("%s Task %d: %d/%d — %s\n", mark, task.Number, task.Score(), task.Weight, task.Title)
		for _, check := range task.Checks {
			checkMark := "✗"
			if check.Passed {
				checkMark = "✓"
			}
			fmt.Printf("    %s %dpt %s\n", checkMark, check.Points, check.Description)
		}
	}
	fmt.Printf("\nScore: %d/%d\n", report.Score(), report.MaxScore())
}

// PrintLabDetails prints detailed information about a lab
func PrintLabDetails(lab labs.Lab) {
	fmt.Printf("\n")
	fmt.Printf("╔═══════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ Lab: %-60s ║\n", lab.Title())
	fmt.Printf("╚═══════════════════════════════════════════════════════════════════╝\n")
	fmt.Printf("\n")
	fmt.Printf("ID:              %s\n", lab.ID())
	fmt.Printf("Category:        %s\n", lab.Category())
	fmt.Printf("Difficulty:      %s\n", lab.Difficulty())
	fmt.Printf("Estimated Time:  %d minutes\n", lab.EstimatedTime())

	tags := lab.Tags()
	if len(tags) > 0 {
		fmt.Printf("Tags:            %s\n", strings.Join(tags, ", "))
	}

	fmt.Printf("\n")
	fmt.Printf("Description:\n")
	fmt.Printf("%s\n", lab.Description())
	fmt.Printf("\n")

	hints := lab.Hints()
	if len(hints) > 0 {
		fmt.Printf("Hints:\n")
		for i, hint := range hints {
			fmt.Printf("  %d. %s\n", i+1, hint)
		}
		fmt.Printf("\n")
	}
}

// Success prints a success message
func Success(message string) {
	fmt.Printf("✓ %s\n", message)
}

// Error prints an error message
func Error(message string) {
	fmt.Printf("✗ %s\n", message)
}

// Info prints an info message
func Info(message string) {
	fmt.Printf("ℹ %s\n", message)
}

// Warning prints a warning message
func Warning(message string) {
	fmt.Printf("⚠ %s\n", message)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
