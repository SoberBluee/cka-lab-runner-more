package labs

import "testing"

func TestExamReportScoring(t *testing.T) {
	report := ExamReport{Tasks: []ExamTaskResult{
		examTask(1, "one", 8,
			examCheck("a", 3, true),
			examCheck("b", 5, false)),
		examTask(2, "two", 7,
			examCheck("c", 7, true)),
	}}
	if report.Score() != 10 || report.MaxScore() != 15 {
		t.Fatalf("score=%d max=%d, want 10/15", report.Score(), report.MaxScore())
	}
}

func TestMockExamWeightsTotal100(t *testing.T) {
	weights := []int{8, 7, 6, 8, 10, 12, 8, 8, 10, 9, 6, 8}
	total := 0
	for _, weight := range weights {
		total += weight
	}
	if total != 100 {
		t.Fatalf("weights total %d, want 100", total)
	}
}
