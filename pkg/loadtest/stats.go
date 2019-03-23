package loadtest

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/expfmt"

	pdto "github.com/prometheus/client_model/go"
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
	AvgTotalClientTime    float64           // Average total time that we were waiting for a single client's interactions/requests to complete.
	AvgAbsTotalClientTime float64           // Average total time waiting for client's interactions/requests to complete, including wait times.
	CountPerClient        float64           // Number of interactions/requests per client.
	AvgTimePerClient      float64           // Average time per interaction/request per client.
	AvgAbsTimePerClient   float64           // Average time per interaction/request per client, including wait times.
	PerSecPerClient       float64           // Interactions/requests per second per client.
	AbsPerSecPerClient    float64           // Interactions/requests per second per client, including wait times.
	PerSec                float64           // Interactions/requests per second overall.
	AbsPerSec             float64           // Interactions/requests per second overall, including wait times.
	FailureRate           float64           // The % of interactions/requests that failed.
	TopErrors             []*FlattenedError // The top 10 errors by error count.
}

// PrometheusStats encapsulates all of the statistics we retrieved from the
// Prometheus endpoints for our target nodes.
type PrometheusStats struct {
	StartTime       time.Time                         // When was the load test started?
	TargetNodeStats map[string][]*NodePrometheusStats // Target node stats organized by hostname.
}

// NodePrometheusStats represents a single test network node's stats from its Prometheus endpoint.
type NodePrometheusStats struct {
	Timestamp      time.Time
	MetricFamilies map[string]*pdto.MetricFamily
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
func AddStatistic(stats *messages.SummaryStats, timeTaken, absTimeTaken time.Duration, err error) {
	timeTakenNs := int64(timeTaken) / int64(time.Nanosecond)
	absTimeTakenNs := int64(absTimeTaken) / int64(time.Nanosecond)
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
	stats.AbsTotalTime += absTimeTakenNs

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
	dest.AbsTotalTime += src.AbsTotalTime
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
	avgAbsTotalClientTime := math.Round(float64(stats.AbsTotalTime) / float64(stats.TotalClients))
	// number of interactions/requests per client
	countPerClient := math.Round(float64(stats.Count) / float64(stats.TotalClients))
	// average time per interaction/request per client
	avgTimePerClient := float64(0)
	avgAbsTimePerClient := float64(0)
	if countPerClient > 0 {
		avgTimePerClient = avgTotalClientTime / countPerClient
		avgAbsTimePerClient = avgAbsTotalClientTime / countPerClient
	}
	// interactions/requests per second per client
	perSecPerClient := float64(0)
	absPerSecPerClient := float64(0)
	if avgTotalClientTime > 0 {
		perSecPerClient = 1 / (avgTimePerClient / float64(time.Second))
		absPerSecPerClient = 1 / (avgAbsTimePerClient / float64(time.Second))
	}
	// interactions/requests per second overall
	perSec := perSecPerClient * float64(stats.TotalClients)
	absPerSec := absPerSecPerClient * float64(stats.TotalClients)
	return &CalculatedStats{
		AvgTotalClientTime:    avgTotalClientTime,
		AvgAbsTotalClientTime: avgAbsTotalClientTime,
		CountPerClient:        countPerClient,
		AvgTimePerClient:      avgTimePerClient,
		AvgAbsTimePerClient:   avgAbsTimePerClient,
		PerSecPerClient:       perSecPerClient,
		AbsPerSecPerClient:    absPerSecPerClient,
		PerSec:                perSec,
		AbsPerSec:             absPerSec,
		FailureRate:           float64(100) * (float64(stats.Errors) / float64(stats.Count)),
		TopErrors:             FlattenedSortedErrors(stats),
	}
}

// LogSummaryStats is a utility function for displaying summary statistics
// through the logging interface. This is useful when working with tm-load-test
// from the command line.
func LogSummaryStats(logger logging.Logger, logPrefix string, stats *messages.SummaryStats) {
	cs := CalculateStats(stats)

	logger.Info(logPrefix+" total clients", "value", stats.TotalClients)
	logger.Info(logPrefix+" per second per client", "value", fmt.Sprintf("%.2f", cs.PerSecPerClient))
	logger.Info(logPrefix+" per second per client (absolute)", "value", fmt.Sprintf("%.2f", cs.AbsPerSecPerClient))
	logger.Info(logPrefix+" per second overall", "value", fmt.Sprintf("%.2f", cs.PerSec))
	logger.Info(logPrefix+" per second overall (absolute)", "value", fmt.Sprintf("%.2f", cs.AbsPerSec))
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
	if err := writer.Write([]string{prefix + " per second per client (absolute)", fmt.Sprintf("%.2f", cs.AbsPerSecPerClient)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " per second overall", fmt.Sprintf("%.2f", cs.PerSec)}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " per second overall (absolute)", fmt.Sprintf("%.2f", cs.AbsPerSec)}); err != nil {
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
	if err := writer.Write([]string{prefix + " min time", (time.Duration(stats.MinTime) * time.Nanosecond).String()}); err != nil {
		return err
	}
	if err := writer.Write([]string{prefix + " max time", (time.Duration(stats.MaxTime) * time.Nanosecond).String()}); err != nil {
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

//
// NodePrometheusStats
//

// GetNodePrometheusStats will perform a blocking GET request to the given URL
// to fetch the text-formatted Prometheus metrics for a particular node, parse
// those metrics, and then either return the parsed metrics or an error.
func GetNodePrometheusStats(c *http.Client, url string) (*NodePrometheusStats, error) {
	stats := &NodePrometheusStats{
		Timestamp:      time.Now(),
		MetricFamilies: nil,
	}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Non-OK response from URL: %s (%d)", resp.Status, resp.StatusCode)
	}

	parser := &expfmt.TextParser{}
	mf, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, err
	}

	stats.MetricFamilies = mf

	return stats, nil
}

//
// PrometheusStats
//

type nodePrometheusStats struct {
	hostID string
	stats  *NodePrometheusStats
}

func collectPrometheusStats(c *http.Client, hostID, nodeURL string, statsc chan nodePrometheusStats, logger logging.Logger) {
	stats, err := GetNodePrometheusStats(c, nodeURL)
	if err == nil {
		statsc <- nodePrometheusStats{
			hostID: hostID,
			stats:  stats,
		}
	} else {
		logger.Error("Failed to retrieve Prometheus stats", "hostID", hostID, "err", err)
	}
}

// RunCollectors will kick off one goroutine per Tendermint node from which
// we're collecting Prometheus stats.
func (ps *PrometheusStats) RunCollectors(cfg *Config, shutdownc, donec chan bool, logger logging.Logger) {
	clients := make([]*http.Client, 0)
	statsc := make(chan nodePrometheusStats, len(cfg.TestNetwork.Targets))
	tickers := make([]*time.Ticker, 0)
	collectorsShutdownc := make([]chan bool, 0)
	logger.Debug("Starting up Prometheus collectors", "count", len(cfg.TestNetwork.Targets))
	ps.StartTime = time.Now()
	wg := &sync.WaitGroup{}
	for _, node := range cfg.TestNetwork.Targets {
		c := &http.Client{
			Timeout: time.Duration(cfg.TestNetwork.PrometheusPollTimeout),
		}
		clients = append(clients, c)
		ticker := time.NewTicker(time.Duration(cfg.TestNetwork.PrometheusPollInterval))
		tickers = append(tickers, ticker)
		collectorShutdownc := make(chan bool, 1)
		collectorsShutdownc = append(collectorsShutdownc, collectorShutdownc)
		wg.Add(1)
		go func(client *http.Client, hostID, nodeURL string, csc chan bool) {
			logger.Debug("Goroutine created for Prometheus collector", "hostID", hostID, "nodeURL", nodeURL)
			// fire off an initial request for stats (prior to first tick)
			collectPrometheusStats(client, hostID, nodeURL, statsc, logger)
		collectorLoop:
			for {
				select {
				case <-ticker.C:
					collectPrometheusStats(client, hostID, nodeURL, statsc, logger)

				case <-csc:
					logger.Debug("Shutting down collector goroutine", "hostID", hostID)
					break collectorLoop
				}
			}
			wg.Done()
		}(c, node.ID, node.GetPrometheusURL(cfg.TestNetwork.PrometheusPort), collectorShutdownc)
	}
	logger.Debug("Prometheus collectors started")

collectorsLoop:
	for {
		select {
		case nodeStats := <-statsc:
			ps.TargetNodeStats[nodeStats.hostID] = append(ps.TargetNodeStats[nodeStats.hostID], nodeStats.stats)

		case <-shutdownc:
			logger.Debug("Shutdown signal received by Prometheus collector loop")
			break collectorsLoop
		}
	}
	for _, ticker := range tickers {
		ticker.Stop()
	}
	for _, csc := range collectorsShutdownc {
		csc <- true
	}
	// wait for all of the collector loops to shut down
	wg.Wait()

	logger.Info("Waiting 10 seconds for network to settle post-test...")
	time.Sleep(10 * time.Second)

	logger.Debug("Doing one final Prometheus stats collection from each node")
	// do one final collection from each node
	for i, node := range cfg.TestNetwork.Targets {
		nodeStats, err := GetNodePrometheusStats(clients[i], node.GetPrometheusURL(cfg.TestNetwork.PrometheusPort))
		if err != nil {
			logger.Error("Failed to fetch Prometheus statistics", "hostID", node.ID, "err", err)
		} else {
			ps.TargetNodeStats[node.ID] = append(ps.TargetNodeStats[node.ID], nodeStats)
		}
	}

	logger.Debug("Prometheus collector loop shut down")
	donec <- true
}

func writeTimeSeriesTargetNodeStats(w io.Writer, startTime time.Time, nodeStats []*NodePrometheusStats) error {
	// first extract a sorted list of the metric families for this node
	familyNames := make(map[string]interface{})
	timestamps := make([]string, 0)
	familySamples := make(map[string][]string)
	for _, sample := range nodeStats {
		added := 0
		for _, mf := range sample.MetricFamilies {
			fn := ""
			// we only want counters and gauges right now
			switch *mf.Type {
			case pdto.MetricType_COUNTER:
				if len(*mf.Help) > 0 {
					fn = *mf.Help
				} else {
					fn = *mf.Name
				}
				familyNames[fn] = nil
				familySamples[fn] = append(familySamples[fn], fmt.Sprintf("%.4f", *(mf.Metric[0].Counter.Value)))
				added++

			case pdto.MetricType_GAUGE:
				if len(*mf.Help) > 0 {
					fn = *mf.Help
				} else {
					fn = *mf.Name
				}
				familyNames[fn] = nil
				familySamples[fn] = append(familySamples[fn], fmt.Sprintf("%.4f", *(mf.Metric[0].Gauge.Value)))
				added++
			}
		}
		if added > 0 {
			timestamps = append(timestamps, fmt.Sprintf("%.2f", sample.Timestamp.Sub(startTime).Seconds()))
		}
	}
	familyNamesSorted := make([]string, 0)
	for name := range familyNames {
		familyNamesSorted = append(familyNamesSorted, name)
	}
	sort.SliceStable(familyNamesSorted[:], func(i, j int) bool {
		return strings.Compare(familyNamesSorted[i], familyNamesSorted[j]) < 0
	})

	// then we write the data to the given file as time series CSV data
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{"Metric Family"}
	header = append(header, timestamps...)
	if err := cw.Write(header); err != nil {
		return err
	}

	// now we write the metric family samples
	for _, name := range familyNamesSorted {
		if err := cw.Write(append([]string{name}, familySamples[name]...)); err != nil {
			return err
		}
	}

	return nil
}

// Dump will write each node's stats to a separate file, named according to the
// host ID, in the given output directory.
func (ps *PrometheusStats) Dump(outputPath string) error {
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return err
	}
	for hostID, nodeStats := range ps.TargetNodeStats {
		outputFile := path.Join(outputPath, fmt.Sprintf("%s.csv", hostID))
		f, err := os.Create(outputFile)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := writeTimeSeriesTargetNodeStats(f, ps.StartTime, nodeStats); err != nil {
			return err
		}
	}
	return nil
}
