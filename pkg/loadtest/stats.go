package loadtest

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/tendermint/networks/internal/logging"

	"github.com/tendermint/networks/pkg/loadtest/messages"
)

// DefaultHistogramBinCount indicates the maximum number of bins in a histogram.
// A value of 100 will give us percentiles.
const DefaultHistogramBinCount int64 = 100

// FlattenedError is the key/value pair from
// `messages.SummaryStats.ErrorsByType` flattened into a structure for easy
// sorting.
type FlattenedError struct {
	ErrorType string
	Count     int64
}

// CalculatedStats includes a number of parameters that can be calculated from a
// `messages.SummaryStats` object.
type CalculatedStats struct {
	AvgTotalClientTime float64           // Average total time that we were waiting for a single client's interactions/requests to complete.
	CountPerClient     float64           // Number of interactions/requests per client.
	AvgTimePerClient   float64           // Average time per interaction/request per client.
	PerSecPerClient    float64           // Interactions/requests per second per client.
	PerSec             float64           // Interactions/requests per second overall.
	FailureRate        float64           // The % of interactions/requests that failed.
	TopErrors          []*FlattenedError // The top 10 errors by error count.
}

// NewSummaryStats instantiates an empty, configured SummaryStats object.
func NewSummaryStats(timeout time.Duration, totalClients int64) *messages.SummaryStats {
	return &messages.SummaryStats{
		TotalClients:  totalClients,
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
	dest.TotalClients += src.TotalClients

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

func FlattenedSortedErrors(stats *messages.SummaryStats) []*FlattenedError {
	errorsByType := make([]*FlattenedError, 0)
	for errStr, count := range stats.ErrorsByType {
		errorsByType = append(errorsByType, &FlattenedError{ErrorType: errStr, Count: count})
	}
	sort.SliceStable(errorsByType[:], func(i, j int) bool {
		return errorsByType[i].Count > errorsByType[j].Count
	})
	return errorsByType
}

func CalculateStats(stats *messages.SummaryStats) *CalculatedStats {
	// average total time for all cumulative interactions/requests per client
	avgTotalClientTime := math.Round(float64(stats.TotalTime) / float64(stats.TotalClients))
	// number of interactions/requests per client
	countPerClient := math.Round(float64(stats.Count) / float64(stats.TotalClients))
	// average time per interaction/request per client
	avgTimePerClient := float64(0)
	if countPerClient > 0 {
		avgTimePerClient = avgTotalClientTime / countPerClient
	}
	// interactions/requests per second per client
	perSecPerClient := float64(0)
	if avgTotalClientTime > 0 {
		perSecPerClient = 1 / (avgTimePerClient / float64(time.Second))
	}
	// interactions/requests per second overall
	perSec := perSecPerClient * float64(stats.TotalClients)
	return &CalculatedStats{
		AvgTotalClientTime: avgTotalClientTime,
		CountPerClient:     countPerClient,
		AvgTimePerClient:   avgTimePerClient,
		PerSecPerClient:    perSecPerClient,
		PerSec:             perSec,
		FailureRate:        float64(100) * (float64(stats.Errors) / float64(stats.Count)),
		TopErrors:          FlattenedSortedErrors(stats),
	}
}

// LogSummaryStats is a utility function for displaying summary statistics
// through the logging interface. This is useful when working with tm-load-test
// from the command line.
func LogSummaryStats(logger logging.Logger, logPrefix string, stats *messages.SummaryStats) {
	cs := CalculateStats(stats)

	logger.Info(logPrefix+" total clients", "value", stats.TotalClients)
	logger.Info(logPrefix+" per second per client", "value", fmt.Sprintf("%.2f", cs.PerSecPerClient))
	logger.Info(logPrefix+" per second overall", "value", fmt.Sprintf("%.2f", cs.PerSec))
	logger.Info(logPrefix+" count", "value", stats.Count)
	logger.Info(logPrefix+" errors", "value", stats.Errors)
	logger.Info(logPrefix+" failure rate (%)", "value", fmt.Sprintf("%.2f", cs.FailureRate))
	logger.Info(logPrefix+" min time", "value", time.Duration(stats.MinTime)*time.Nanosecond)
	logger.Info(logPrefix+" max time", "value", time.Duration(stats.MaxTime)*time.Nanosecond)

	for i := 0; i < 10 && i < len(cs.TopErrors); i++ {
		logger.Info(logPrefix+fmt.Sprintf(" top error %d", i+1), cs.TopErrors[i].ErrorType, cs.TopErrors[i].Count)
	}
}

// LogStats is a utility function to log the given statistics for easy reading,
// especially from the command line.
func LogStats(logger logging.Logger, stats *messages.CombinedStats) {
	logger.Info("")
	logger.Info("INTERACTION STATISTICS")
	LogSummaryStats(logger, "Interactions", stats.Interactions)

	logger.Info("")
	logger.Info("REQUEST STATISTICS")
	for reqName, reqStats := range stats.Requests {
		logger.Info(reqName)
		LogSummaryStats(logger, "  Requests", reqStats)
		logger.Info("")
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

// WriteSummaryStats will write the given summary statistics using the specified
// CSV writer.
func WriteSummaryStats(writer *csv.Writer, indentCount int, linePrefix string, stats *messages.SummaryStats) error {
	cs := CalculateStats(stats)
	indent := strings.Repeat(" ", indentCount)
	prefix := indent + linePrefix
	if err := writer.Write([]string{prefix + " total clients", fmt.Sprintf("%d", stats.TotalClients)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " per second per client", fmt.Sprintf("%.2f", cs.PerSecPerClient)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " per second overall", fmt.Sprintf("%.2f", cs.PerSec)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " count", fmt.Sprintf("%d", stats.Count)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " errors", fmt.Sprintf("%d", stats.Errors)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " failure rate (%)", fmt.Sprintf("%.2f", cs.FailureRate)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " min time", fmt.Sprintf("%s", time.Duration(stats.MinTime)*time.Nanosecond)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " max time", fmt.Sprintf("%s", time.Duration(stats.MaxTime)*time.Nanosecond)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " top errors:"}); err != nil {
		return err
	}
	for i := 0; i < len(cs.TopErrors) && i < 10; i++ {
		if err := writer.Write([]string{indent + "  " + cs.TopErrors[i].ErrorType, fmt.Sprintf("%d", cs.TopErrors[i].Count)}); err != nil {
			return err
		}
	}
	if err := writer.Write([]string{prefix + " response time histogram (milliseconds/count):"}); err != nil {
		return err
	}
	for bin := int64(0); bin <= stats.ResponseTimes.Timeout; bin += stats.ResponseTimes.BinSize {
		if err := writer.Write([]string{
			indent + "  " + fmt.Sprintf("%d", time.Duration(bin)/time.Millisecond),
			fmt.Sprintf("%d", stats.ResponseTimes.TimeBins[bin]),
		}); err != nil {
			return err
		}
	}
	return nil
}

// WriteCombinedStats will write the given combined statistics using the
// specified writer.
func WriteCombinedStats(writer io.Writer, stats *messages.CombinedStats) error {
	cw := csv.NewWriter(writer)
	defer cw.Flush()

	if err := cw.Write([]string{"INTERACTION STATISTICS"}); err != nil {
		return err
	}
	if err := WriteSummaryStats(cw, 0, "Interactions", stats.Interactions); err != nil {
		return err
	}

	if err := cw.Write([]string{}); err != nil {
		return err
	}
	if err := cw.Write([]string{"REQUEST STATISTICS"}); err != nil {
		return err
	}
	i := 0
	for reqName, reqStats := range stats.Requests {
		if i > 0 {
			if err := cw.Write([]string{}); err != nil {
				return err
			}
		}
		if err := cw.Write([]string{reqName}); err != nil {
			return err
		}
		if err := WriteSummaryStats(cw, 2, "Requests", reqStats); err != nil {
			return err
		}
		i++
	}
	return nil
}

// WriteCombinedStatsToFile will write the given stats object to the specified
// output CSV file. If the given output path does not exist, it will be created.
func WriteCombinedStatsToFile(outputFile string, stats *messages.CombinedStats) error {
	outputPath := path.Dir(outputFile)
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return err
	}
	f, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteCombinedStats(f, stats)
}
