package loadtest

import (
	"fmt"
	"math"
	"time"
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
	Count        int64          // Total number of interactions/requests.
	Errors       int64          // How many errors occurred.
	TotalTime    int64          // Interaction time summed across all interactions/requests (milliseconds).
	AvgTime      float64        // Average interaction/request time (milliseconds).
	MinTime      int64          // Minimum interaction/request time (milliseconds).
	MaxTime      int64          // Maximum interaction/request time (milliseconds).
	TimeBins     map[int]int    // Counts of interaction/request times (histogram).
	ErrorsByType map[string]int // Counts of different errors by type of error.

	timeout  int64 // The maximum possible range of the histogram.
	binSize  int64 // The size of a histogram bin (milliseconds).
	binCount int   // The number of bins in TimeBins.
}

// ClientSummaryStats gives us statistics for a particular client.
type ClientSummaryStats struct {
	Interactions *SummaryStats            // Stats over all of the interactions.
	Requests     map[string]*SummaryStats // A mapping of each request ID to a summary of its stats.
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
	binCount := DefaultStatsHistogramBins
	bins := make(map[int]int)
	binSize := timeoutMs / int64(binCount)
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
		timeout:      timeoutMs,
		binSize:      binSize,
		binCount:     binCount + 1,
	}
}

// Add will add a new interaction time (milliseconds) and perform the relevant
// calculations.
func (s *SummaryStats) Add(t int64, errs ...error) {
	// cap the value at the timeout
	if t > s.timeout {
		t = s.timeout
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

	bin := (t / s.binSize) * s.binSize
	if bin <= s.timeout {
		s.TimeBins[int(bin)]++
	}
}

// AddNano calls Add assuming that the given time is in nanoseconds, and thus
// needs to be divided by 1000 before being added.
func (s *SummaryStats) AddNano(t int64, errs ...error) {
	s.Add(t/1000, errs...)
}

// TimeAndAdd executes the given function, tracking how long it takes to
// execute and whether it returns an error, and adds
func (s *SummaryStats) TimeAndAdd(req func() error) {
	rs := TimeRequest(req)
	s.Add(rs.TimeTaken, rs.Err)
}

// Combine will add the stats from the given summary into this one. Assumes that
// `o` has the exact same time bin configuration as `i`.
func (s *SummaryStats) Combine(o *SummaryStats) {
	s.TotalTime += o.TotalTime
	s.Count += o.Count
	s.Errors += o.Errors

	if o.MinTime < s.MinTime {
		s.MinTime = o.MinTime
	}
	if o.MaxTime > s.MaxTime {
		s.MaxTime = o.MaxTime
	}

	// combine the counts from the bins
	for bin := int64(0); bin <= s.timeout; bin += s.binSize {
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
	s.AvgTime = float64(s.TotalTime) / float64(s.Count)
}

// PrintTimeBins is useful for debugging purposes.
func (s *SummaryStats) PrintTimeBins() {
	i := 0
	for bin := int64(0); bin <= s.timeout; bin += s.binSize {
		fmt.Printf("%d:\t%d\t", bin, s.TimeBins[int(bin)])
		i++
		if (i % 10) == 0 {
			fmt.Printf("\n")
		}
	}
}

//
// ClientSummaryStats
//

// Combine will combine the given client summary stats with this one.
func (c *ClientSummaryStats) Combine(o *ClientSummaryStats) {
	c.Interactions.Combine(o.Interactions)
	for reqID, stats := range o.Requests {
		if _, ok := c.Requests[reqID]; ok {
			// if we've already seen this request before, combine them
			c.Requests[reqID].Combine(stats)
		} else {
			// if we haven't, make a copy of the other's stats
			c.Requests[reqID] = &(*stats)
		}
	}
}
