package loadtest_test

import (
	"testing"
	"time"

	"github.com/tendermint/networks/pkg/loadtest"
)

func TestSummaryStats(t *testing.T) {
	s := loadtest.NewSummaryStats(10 * time.Second)
	if len(s.TimeBins) != loadtest.DefaultStatsHistogramBins+1 {
		t.Fatalf("Expected %d time bins in summary stats, but got %d", loadtest.DefaultStatsHistogramBins, len(s.TimeBins))
	}
	testCases := []struct {
		timeTaken int64
		totalTime int64
		bin       int
		binCount  int
	}{
		{500, 500, 500, 1},
		{2000, 2500, 2000, 1},
		{2050, 4550, 2000, 2},
		{2800, 7350, 2800, 1},
		{3000, 10350, 3000, 1},
		{3010, 13360, 3000, 2},
		{3090, 16450, 3000, 3}, // testing truncation to the lower bin
	}
	for _, tc := range testCases {
		s.Add(tc.timeTaken, nil)

		if tc.totalTime != s.TotalTime {
			t.Fatal("Expected total time to be", tc.totalTime, "but was", s.TotalTime)
		}
		count, ok := s.TimeBins[tc.bin]
		if !ok {
			t.Fatal("Expected bin", tc.bin, "to be present, but it was not")
		}
		if tc.binCount != count {
			t.Fatal("Expected bin", tc.bin, "to contain value", tc.binCount, ", but was", count)
		}
	}
}
