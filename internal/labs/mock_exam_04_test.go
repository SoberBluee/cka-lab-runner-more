package labs

import "testing"

func TestMockExam04WeightsTotal100(t *testing.T) {
	weights := []int{8, 10, 10, 8, 10, 10, 8, 10, 10, 10, 6}
	total := 0
	for _, weight := range weights {
		total += weight
	}
	if total != 100 {
		t.Fatalf("weights total %d, want 100", total)
	}
}
