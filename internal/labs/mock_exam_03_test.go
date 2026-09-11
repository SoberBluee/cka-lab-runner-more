package labs

import "testing"

func TestMockExam03WeightsTotal100(t *testing.T) {
	weights := []int{8, 10, 10, 12, 12, 12, 12, 8, 8, 8}
	total := 0
	for _, weight := range weights {
		total += weight
	}
	if total != 100 {
		t.Fatalf("weights total %d, want 100", total)
	}
}
