package loadtest

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/tendermint/networks/internal/logging"

	"github.com/tendermint/networks/pkg/loadtest/messages"
)

// DefaultHistogramBinCount indicates the maximum number of bins in a histogram.
// A value of 100 will give us percentiles.
const DefaultHistogramBinCount int64 = 100

type flattenedError struct {
	errorType string
	count     int64
}

// NewSummaryStats instantiates an empty, configured SummaryStats object.
func NewSummaryStats(timeout time.Duration) *messages.SummaryStats {
	return &messages.SummaryStats{
		ErrorsByType:  make(map[string]int64),
		ResponseTimes: NewResponseTimeHistogram(timeout),
	}
}

// AddStatistic adds a single statistic to the given SummaryStats object.
func AddStatistic(stats *messages.SummaryStats, timeTaken time.Duration, err error) {
	timeTakenNs := int64(timeTaken) / int64(time.Nanosecond)
	// we cap the time taken at the response timeout
	if timeTakenNs > stats.ResponseTimes.Timeout {
		timeTakenNs = stats.ResponseTimes.Timeout
	}

	if stats.Count == 0 {
		stats.MinTime = timeTakenNs
		stats.MaxTime = timeTakenNs
	} else {
		if timeTakenNs < stats.MinTime {
			stats.MinTime = timeTakenNs
		}
		if timeTakenNs > stats.MaxTime {
			stats.MaxTime = timeTakenNs
		}
	}

	stats.Count++
	stats.TotalTime += timeTakenNs

	if err != nil {
		stats.Errors++
		errType := fmt.Sprintf("%s", err)
		if _, ok := stats.ErrorsByType[errType]; !ok {
			stats.ErrorsByType[errType] = 1
		} else {
			stats.ErrorsByType[errType]++
		}
	}

	bin := int64(0)
	if timeTakenNs > 0 {
		bin = stats.ResponseTimes.BinSize * (timeTakenNs / stats.ResponseTimes.BinSize)
	}
	stats.ResponseTimes.TimeBins[bin]++
}

// MergeSummaryStats will merge the given source stats into the destination
// stats, modifying the destination stats object.
func MergeSummaryStats(dest, src *messages.SummaryStats) {
	if src.Count == 0 {
		return
	}

	dest.Count += src.Count
	dest.Errors += src.Errors
	dest.TotalTime += src.TotalTime

	if src.MinTime < dest.MinTime {
		dest.MinTime = src.MinTime
	}
	if src.MaxTime > dest.MaxTime {
		dest.MaxTime = src.MaxTime
	}

	// merge ErrorsByType
	for errType, srcCount := range src.ErrorsByType {
		_, ok := dest.ErrorsByType[errType]
		if ok {
			dest.ErrorsByType[errType] += srcCount
		} else {
			dest.ErrorsByType[errType] = srcCount
		}
	}

	// merge response times histogram (assuming all bins are precisely the same
	// for the stats)
	for bin, srcCount := range src.ResponseTimes.TimeBins {
		dest.ResponseTimes.TimeBins[bin] += srcCount
	}
}

// MergeCombinedStats will merge the given src CombinedStats object's contents
// into the dest object.
func MergeCombinedStats(dest, src *messages.CombinedStats) {
	dest.TotalInteractionTime += src.TotalInteractionTime
	MergeSummaryStats(dest.Interactions, src.Interactions)
	for srcReqName, srcReqStats := range src.Requests {
		MergeSummaryStats(dest.Requests[srcReqName], srcReqStats)
	}
}

// LogStats is a utility function to log the given statistics for easy reading.
func LogStats(logger logging.Logger, stats *messages.CombinedStats) {
	avgTime := math.Round(float64(stats.Interactions.TotalTime) / float64(stats.Interactions.Count))
	intsPerSec := float64(0)
	if avgTime > 0 {
		// avgTime is ns - needs to be converted to seconds
		intsPerSec = 1 / (avgTime / float64(time.Second))
	}
	totalAvgTime := math.Round(float64(stats.TotalInteractionTime) / float64(stats.Interactions.Count))
	totalIntsPerSec := float64(0)
	if totalAvgTime > 0 {
		totalIntsPerSec = 1 / (totalAvgTime / float64(time.Second))
	}
	logger.Info("Stats: Interactions", "TotalInteractionTime", time.Duration(stats.TotalInteractionTime)*time.Nanosecond)
	logger.Info("Stats: Interactions", "Count", stats.Interactions.Count)
	logger.Info("Stats: Interactions", "Errors", stats.Interactions.Errors)
	logger.Info("Stats: Interactions", "TotalTime", time.Duration(stats.Interactions.TotalTime)*time.Nanosecond)
	logger.Info("Stats: Interactions", "MinTime", time.Duration(stats.Interactions.MinTime)*time.Nanosecond)
	logger.Info("Stats: Interactions", "MaxTime", time.Duration(stats.Interactions.MaxTime)*time.Nanosecond)
	logger.Info("Stats: Interactions", "AvgTime", time.Duration(avgTime)*time.Nanosecond)
	logger.Info("Stats: Interactions", "TotalAvgTime", time.Duration(totalAvgTime)*time.Nanosecond)
	logger.Info("Stats: Interactions", "IntsPerSec", intsPerSec)
	logger.Info("Stats: Interactions", "TotalIntsPerSec", totalIntsPerSec)

	// work out the top 10 errors
	errorsByType := make([]flattenedError, 0)
	for errStr, count := range stats.Interactions.ErrorsByType {
		errorsByType = append(errorsByType, flattenedError{errorType: errStr, count: count})
	}
	sort.SliceStable(errorsByType[:], func(i, j int) bool {
		return errorsByType[i].count > errorsByType[j].count
	})
	for i := 0; i < 10 && i < len(errorsByType); i++ {
		logger.Info("Stats: Interactions - Errors", errorsByType[i].errorType, errorsByType[i].count)
	}
}

// NewResponseTimeHistogram instantiates an empty response time histogram with
// the given timeout as an upper bound on the size of the histogram.
func NewResponseTimeHistogram(timeout time.Duration) *messages.ResponseTimeHistogram {
	timeoutNs := int64(timeout) / int64(time.Nanosecond)
	binSize := int64(math.Ceil(float64(timeoutNs) / float64(DefaultHistogramBinCount)))
	timeBins := make(map[int64]int64)
	for i := int64(0); i <= DefaultHistogramBinCount; i++ {
		timeBins[i*binSize] = 0
	}
	return &messages.ResponseTimeHistogram{
		Timeout:  timeoutNs,
		BinSize:  binSize,
		BinCount: DefaultHistogramBinCount + 1,
		TimeBins: timeBins,
	}
}
