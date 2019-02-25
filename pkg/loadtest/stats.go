package loadtest

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/tendermint/networks/internal/logging"
)

// DefaultStatsHistogramBins specifies the number of bins that make up a
// histogram. Setting to 100 gives us percentiles.
const DefaultStatsHistogramBins = 100

// RequestStats gives us per-request statistics.
type RequestStats struct {
	TimeTaken int64 // The amount of time taken to complete this request (milliseconds).
	Err       error // For if any error occurred during a particular request.
}

// SummaryStats represents a collection of statistics relevant to multiple
// interactions or requests.
type SummaryStats struct {
	Count        int64          `json:"count"`        // Total number of interactions/requests.
	Errors       int64          `json:"errors"`       // How many errors occurred.
	TotalTime    int64          `json:"totalTime"`    // Interaction time summed across all interactions/requests (milliseconds).
	AvgTime      float64        `json:"avgTime"`      // Average interaction/request time (milliseconds).
	MinTime      int64          `json:"minTime"`      // Minimum interaction/request time (milliseconds).
	MaxTime      int64          `json:"maxTime"`      // Maximum interaction/request time (milliseconds).
	TimeBins     map[int]int    `json:"timeBins"`     // Counts of interaction/request times (histogram).
	ErrorsByType map[string]int `json:"errorsByType"` // Counts of different errors by type of error.

	Timeout  int64 `json:"timeout"`  // The maximum possible range of the histogram.
	BinSize  int64 `json:"binSize"`  // The size of a histogram bin (milliseconds).
	BinCount int   `json:"binCount"` // The number of bins in TimeBins.
}

// ClientSummaryStats gives us statistics for a particular client.
type ClientSummaryStats struct {
	Interactions *SummaryStats            `json:"interactions"` // Stats over all of the interactions.
	Requests     map[string]*SummaryStats `json:"requests"`     // A mapping of each request ID to a summary of its stats.
}

//
// RequestStats
//

// TimeRequest is a utility function that assists with executing the given
// request function and timing how long it takes to return. This function will
// return a `RequestStats` object containing the details of the request's
// execution.
func TimeRequest(req func() error) *RequestStats {
	startTime := time.Now().UnixNano()
	err := req()
	return &RequestStats{
		TimeTaken: int64(math.Round((float64(time.Now().UnixNano()) - float64(startTime)) / 1000.0)),
		Err:       err,
	}
}

//
// SummaryStats
//

// NewSummaryStats creates a summary stats tracker with the given maximum
// timeout for an interaction/request.
func NewSummaryStats(timeout time.Duration) *SummaryStats {
	timeoutMs := int64(math.Round(float64(timeout / time.Millisecond)))
	if timeoutMs == 0 {
		panic("timeout cannot be 0ms")
	}
	binCount := DefaultStatsHistogramBins
	bins := make(map[int]int)
	binSize := timeoutMs / int64(binCount)
	if binSize == 0 {
		panic(fmt.Sprintf("timeout must be at least %dms", binCount))
	}
	for i := 0; i < binCount+1; i++ {
		bins[i*int(binSize)] = 0
	}
	return &SummaryStats{
		Count:        0,
		Errors:       0,
		TotalTime:    0,
		AvgTime:      0,
		MinTime:      0,
		MaxTime:      0,
		TimeBins:     bins,
		ErrorsByType: make(map[string]int),
		Timeout:      timeoutMs,
		BinSize:      binSize,
		BinCount:     binCount + 1,
	}
}

// Add will add a new interaction time (milliseconds) and perform the relevant
// calculations.
func (s *SummaryStats) Add(t int64, errs ...error) {
	// cap the value at the timeout
	if t > s.Timeout {
		t = s.Timeout
	}

	if s.Count == 0 {
		s.MinTime = t
		s.MaxTime = t
	} else {
		if t < s.MinTime {
			s.MinTime = t
		}
		if t > s.MaxTime {
			s.MaxTime = t
		}
	}

	s.TotalTime += t
	s.Count++

	if len(errs) > 0 && errs[0] != nil {
		errStr := errs[0].Error()
		if _, ok := s.ErrorsByType[errStr]; !ok {
			s.ErrorsByType[errStr] = 1
		} else {
			s.ErrorsByType[errStr]++
		}
		s.Errors++
	}

	bin := (t / s.BinSize) * s.BinSize
	if bin <= s.Timeout {
		s.TimeBins[int(bin)]++
	}
}

// AddNano calls Add assuming that the given time is in nanoseconds, and thus
// needs to be divided by 1,000,000 before being added.
func (s *SummaryStats) AddNano(t int64, errs ...error) {
	s.Add(t/1000000, errs...)
}

// TimeAndAdd executes the given function, tracking how long it takes to
// execute and whether it returns an error, and adds
func (s *SummaryStats) TimeAndAdd(req func() error) {
	rs := TimeRequest(req)
	s.Add(rs.TimeTaken, rs.Err)
}

// Merge will add the stats from the given summary into this one. Assumes that
// `o` has the exact same time bin configuration as `i`.
func (s *SummaryStats) Merge(o *SummaryStats) {
	// if we have no transactions yet, use the other summary's min/max stats
	if s.Count == 0 {
		s.MinTime = o.MinTime
		s.MaxTime = o.MaxTime
	} else {
		if o.MinTime < s.MinTime {
			s.MinTime = o.MinTime
		}
		if o.MaxTime > s.MaxTime {
			s.MaxTime = o.MaxTime
		}
	}

	s.TotalTime += o.TotalTime
	s.Count += o.Count
	s.Errors += o.Errors

	// combine the counts from the bins
	for bin := int64(0); bin <= s.Timeout; bin += s.BinSize {
		s.TimeBins[int(bin)] += o.TimeBins[int(bin)]
	}

	// combine the counts from the error bins
	for errType, count := range o.ErrorsByType {
		if _, ok := s.ErrorsByType[errType]; !ok {
			s.ErrorsByType[errType] = count
		} else {
			s.ErrorsByType[errType] += count
		}
	}
}

// Compute will calculate any remaining stats that weren't calculated on-the-fly
// during the Add function calls.
func (s *SummaryStats) Compute() {
	if s.Count > 0 {
		s.AvgTime = float64(s.TotalTime) / float64(s.Count)
	} else {
		s.AvgTime = 0
	}
}

// PrintTimeBins is useful for debugging purposes.
func (s *SummaryStats) PrintTimeBins() {
	i := 0
	for bin := int64(0); bin <= s.Timeout; bin += s.BinSize {
		fmt.Printf("%d:\t%d\t", bin, s.TimeBins[int(bin)])
		i++
		if (i % 10) == 0 {
			fmt.Printf("\n")
		}
	}
}

// Equals will compare this summary stats object to the given one and, if a
// logger is specified it will be used to output debug information about which
// fields are different between the two objects.
func (s *SummaryStats) Equals(o *SummaryStats, loggers ...logging.Logger) bool {
	equal := true
	logger := logging.NewNoopLogger()
	if len(loggers) > 0 {
		logger = loggers[0]
	}

	if s.Count != o.Count {
		logger.Debug("Field mismatch", "s.Count", s.Count, "o.Count", o.Count)
		equal = false
	}
	if s.Errors != o.Errors {
		logger.Debug("Field mismatch", "s.Errors", s.Errors, "o.Errors", o.Errors)
		equal = false
	}
	if s.TotalTime != o.TotalTime {
		logger.Debug("Field mismatch", "s.TotalTime", s.TotalTime, "o.TotalTime", o.TotalTime)
		equal = false
	}
	if s.AvgTime != o.AvgTime {
		logger.Debug("Field mismatch", "s.AvgTime", s.AvgTime, "o.AvgTime", o.AvgTime)
		equal = false
	}
	if s.MinTime != o.MinTime {
		logger.Debug("Field mismatch", "s.MinTime", s.MinTime, "o.MinTime", o.MinTime)
		equal = false
	}
	if s.MaxTime != o.MaxTime {
		logger.Debug("Field mismatch", "s.MaxTime", s.MaxTime, "o.MaxTime", o.MaxTime)
		equal = false
	}
	if s.Timeout != o.Timeout {
		logger.Debug("Field mismatch", "s.Timeout", s.Timeout, "o.Timeout", o.Timeout)
		equal = false
	}
	if s.BinSize != o.BinSize {
		logger.Debug("Field mismatch", "s.BinSize", s.BinSize, "o.BinSize", o.BinSize)
		equal = false
	}
	if s.BinCount != o.BinCount {
		logger.Debug("Field mismatch", "s.BinCount", s.BinCount, "o.BinCount", o.BinCount)
		equal = false
	}

	if !s.timeBinsEqual(o.TimeBins) {
		logger.Debug("Field mismatch", "s.TimeBins", s.TimeBins, "o.TimeBins", o.TimeBins)
		equal = false
	}
	if !s.errorsByTypeEqual(o.ErrorsByType) {
		logger.Debug("Field mismatch", "s.ErrorsByType", s.ErrorsByType, "o.ErrorsByType", o.ErrorsByType)
		equal = false
	}

	return equal
}

func (s *SummaryStats) timeBinsEqual(o map[int]int) bool {
	if len(s.TimeBins) != len(o) {
		return false
	}
	for bin, count := range s.TimeBins {
		ocount, ok := o[bin]
		if !ok {
			return false
		}
		if ocount != count {
			return false
		}
	}
	return true
}

func (s *SummaryStats) errorsByTypeEqual(o map[string]int) bool {
	if len(s.ErrorsByType) != len(o) {
		return false
	}
	for err, count := range s.ErrorsByType {
		ocount, ok := o[err]
		if !ok {
			return false
		}
		if ocount != count {
			return false
		}
	}
	return true
}

// SummaryCSV compiles the summary statistics into a line that can be written to
// a CSV file.
func (s *SummaryStats) SummaryCSV() []string {
	return []string{
		fmt.Sprintf("%d", s.Errors),
		fmt.Sprintf("%.3f", s.AvgTime),
		fmt.Sprintf("%d", s.MinTime),
		fmt.Sprintf("%d", s.MaxTime),
		fmt.Sprintf("%d", s.Count),
	}
}

func (s *SummaryStats) String() string {
	return fmt.Sprintf("SummaryStats{Count: %d, Errors: %d, TotalTime: %d, "+
		"AvgTime: %.3f, MinTime: %d, MaxTime: %d, TimeBins: %v, ErrorsByType: %v, "+
		"Timeout: %d, BinSize: %d, BinCount: %d}",
		s.Count,
		s.Errors,
		s.TotalTime,
		s.AvgTime,
		s.MinTime,
		s.MaxTime,
		s.timeBinsToString(),
		s.errorsByTypeToString(),
		s.Timeout,
		s.BinSize,
		s.BinCount,
	)
}

func (s *SummaryStats) timeBinsToString() string {
	res := ""
	for bin := int64(0); bin < s.Timeout; bin += s.BinSize {
		if bin > 0 {
			res = res + ", "
		}
		res = res + fmt.Sprintf("%d:%d", bin, s.TimeBins[int(bin)])
	}
	return res
}

func (s *SummaryStats) errorsByTypeToString() string {
	res := ""
	i := 0
	for err, count := range s.ErrorsByType {
		if i > 0 {
			res = res + ", "
		}
		res = res + fmt.Sprintf("\"%s\": %d", err, count)
		i++
	}
	return res
}

//
// ClientSummaryStats
//

// NewClientSummaryStats creates and initialises an empty ClientSummaryStats
// object with the given interaction timeout.
func NewClientSummaryStats(itimeout time.Duration) *ClientSummaryStats {
	return &ClientSummaryStats{
		Interactions: NewSummaryStats(itimeout),
		Requests:     make(map[string]*SummaryStats),
	}
}

// Merge will combine the given client summary stats with this one.
func (c *ClientSummaryStats) Merge(o *ClientSummaryStats) {
	c.Interactions.Merge(o.Interactions)
	for reqID, stats := range o.Requests {
		if _, ok := c.Requests[reqID]; ok {
			// if we've already seen this request before, combine them
			c.Requests[reqID].Merge(stats)
		} else {
			// if we haven't, make a copy of the other's stats
			c.Requests[reqID] = NewSummaryStats(time.Duration(o.Requests[reqID].Timeout) * time.Millisecond)
			*c.Requests[reqID] = *o.Requests[reqID]
		}
	}
}

// Compute does a recursive Compute call on internal objects.
func (c *ClientSummaryStats) Compute() {
	c.Interactions.Compute()
	for _, rs := range c.Requests {
		rs.Compute()
	}
}

// Equals does a deep comparison of this object to `o`, optionally using a
// logger to output more detailed (debug-level) information on what the
// differences are between the two structures.
func (c *ClientSummaryStats) Equals(o *ClientSummaryStats, loggers ...logging.Logger) bool {
	logger := logging.NewNoopLogger()
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	logger.PushFields()

	logger.SetField("field", "Interactions")
	equal := c.Interactions.Equals(o.Interactions, logger)

	logger.SetField("field", "Requests")
	if len(c.Requests) != len(o.Requests) {
		equal = false
		logger.Debug(
			"c.Requests and o.Requests have different sizes",
			"len(c.Requests)", len(c.Requests),
			"len(o.Requests)", len(o.Requests),
		)
	}
	for reqID, req := range c.Requests {
		logger.SetField("reqID", reqID)
		oreq, ok := o.Requests[reqID]
		if !ok {
			logger.Debug("o.Requests is missing entry", "reqID", reqID)
			equal = false
		} else {
			if !req.Equals(oreq, logger) {
				equal = false
			}
		}
	}

	logger.PopFields()
	return equal
}

// WriteSummary will output CSV data using the given writer.
func (c *ClientSummaryStats) WriteSummary(w io.Writer) error {
	headings := []string{"Description", "Errors", "Avg Time", "Min Time", "Max Time", "Total Count"}
	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write(headings); err != nil {
		return err
	}
	row := []string{"Interactions"}
	row = append(row, c.Interactions.SummaryCSV()...)
	if err := csvWriter.Write(row); err != nil {
		return err
	}
	requestIDs := []string{}
	for rid := range c.Requests {
		requestIDs = append(requestIDs, rid)
	}
	sort.Slice(requestIDs, func(i, j int) bool {
		return strings.Compare(requestIDs[i], requestIDs[j]) < 0
	})
	for _, rid := range requestIDs {
		row = []string{rid}
		row = append(row, c.Requests[rid].SummaryCSV()...)
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return nil
}
