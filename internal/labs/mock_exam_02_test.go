package labs

import "testing"

func TestMockExam02WeightsTotal100(t *testing.T) {
	weights := []int{8, 7, 12, 8, 6, 8, 10, 9, 8, 6, 10, 8}
	total := 0
	for _, weight := range weights {
		total += weight
	}
	if total != 100 {
		t.Fatalf("weights total %d, want 100", total)
	}
}
