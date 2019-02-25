package loadtest_test

import (
	"testing"
	"time"

	"github.com/tendermint/networks/internal/logging"
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
	for i, tc := range testCases {
		s.Add(tc.timeTaken, nil)

		if tc.totalTime != s.TotalTime {
			t.Errorf("Test case %d: Expected total time to be %d but was %d", i, tc.totalTime, s.TotalTime)
		}
		count, ok := s.TimeBins[tc.bin]
		if !ok {
			t.Errorf("Test case %d: Expected bin %d to be present, but it was not", i, tc.bin)
		}
		if tc.binCount != count {
			t.Errorf("Test case %d: Expected bin %d to contain value %d, but was %d", i, tc.bin, tc.binCount, count)
		}
	}
}

func TestClientSummaryStatsMerge(t *testing.T) {
	testCases := []struct {
		summaries []*loadtest.ClientSummaryStats
		expected  *loadtest.ClientSummaryStats
	}{
		// TEST CASE 1: Single summary in
		{
			summaries: []*loadtest.ClientSummaryStats{
				&loadtest.ClientSummaryStats{
					Interactions: &loadtest.SummaryStats{
						Count:     10,
						Errors:    0,
						TotalTime: 10000,
						AvgTime:   1000.0,
						MinTime:   1000,
						MaxTime:   1000,
						TimeBins: map[int]int{
							0: 0, 1000: 10, 2000: 0, 3000: 0, 4000: 0, 5000: 0,
						},
						ErrorsByType: map[string]int{},
						Timeout:      5000,
						BinSize:      1000,
						BinCount:     6,
					},
					Requests: map[string]*loadtest.SummaryStats{},
				},
			},
			expected: &loadtest.ClientSummaryStats{
				Interactions: &loadtest.SummaryStats{
					Count:     10,
					Errors:    0,
					TotalTime: 10000,
					AvgTime:   1000.0,
					MinTime:   1000,
					MaxTime:   1000,
					TimeBins: map[int]int{
						0: 0, 1000: 10, 2000: 0, 3000: 0, 4000: 0, 5000: 0,
					},
					ErrorsByType: map[string]int{},
					Timeout:      5000,
					BinSize:      1000,
					BinCount:     6,
				},
				Requests: map[string]*loadtest.SummaryStats{},
			},
		},
		// TEST CASE 2: Two summaries
		{
			summaries: []*loadtest.ClientSummaryStats{
				&loadtest.ClientSummaryStats{
					Interactions: &loadtest.SummaryStats{
						Count:     10,
						Errors:    0,
						TotalTime: 10000,
						AvgTime:   1000.0,
						MinTime:   1000,
						MaxTime:   1000,
						TimeBins: map[int]int{
							0: 0, 1000: 10, 2000: 0, 3000: 0, 4000: 0, 5000: 0,
						},
						ErrorsByType: map[string]int{},
						Timeout:      5000,
						BinSize:      1000,
						BinCount:     6,
					},
					Requests: map[string]*loadtest.SummaryStats{},
				},
				&loadtest.ClientSummaryStats{
					Interactions: &loadtest.SummaryStats{
						Count:     10,
						Errors:    0,
						TotalTime: 20000,
						AvgTime:   2000.0,
						MinTime:   2000,
						MaxTime:   2000,
						TimeBins: map[int]int{
							0: 0, 1000: 0, 2000: 10, 3000: 0, 4000: 0, 5000: 0,
						},
						ErrorsByType: map[string]int{},
						Timeout:      5000,
						BinSize:      1000,
						BinCount:     6,
					},
					Requests: map[string]*loadtest.SummaryStats{},
				},
			},
			expected: &loadtest.ClientSummaryStats{
				Interactions: &loadtest.SummaryStats{
					Count:     20,
					Errors:    0,
					TotalTime: 30000,
					AvgTime:   1500.0,
					MinTime:   1000,
					MaxTime:   2000,
					TimeBins: map[int]int{
						0: 0, 1000: 10, 2000: 10, 3000: 0, 4000: 0, 5000: 0,
					},
					ErrorsByType: map[string]int{},
					Timeout:      5000,
					BinSize:      1000,
					BinCount:     6,
				},
				Requests: map[string]*loadtest.SummaryStats{},
			},
		},
	}
	logger := logging.NewLogrusLogger("test")
	for i, tc := range testCases {
		actual := &loadtest.ClientSummaryStats{
			Interactions: &loadtest.SummaryStats{
				TimeBins: map[int]int{
					0: 0, 1000: 0, 2000: 0, 3000: 0, 4000: 0, 5000: 0,
				},
				Timeout:  5000,
				BinSize:  1000,
				BinCount: 6,
			},
			Requests: map[string]*loadtest.SummaryStats{},
		}
		for _, stats := range tc.summaries {
			actual.Merge(stats)
		}
		actual.Compute()

		if !tc.expected.Equals(actual, logger) {
			t.Errorf("Test case %d: Expected %v but got %v", i, tc.expected, actual)
		}
	}
}
