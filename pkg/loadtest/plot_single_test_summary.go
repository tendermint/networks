package loadtest

import (
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/tendermint/networks/pkg/loadtest/messages"
)

// SingleTestSummaryPlot is an HTML page template that facilitates plotting the
// summary results (using Chart.js) of a single load test.
const SingleTestSummaryPlot = `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Load Test Summary</title>
	<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/bulma/0.7.4/css/bulma.min.css">
</head>
<body>
	<section class="section">
	<div class="container">
		<h1 class="title is-1">Load Test Summary</h1>
		
		<table class="table is-striped is-hoverable">
		<thead>
				<tr>
					<th>Parameter</th>
					<th>Value</th>
					<th></th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<td>Client type</td>
					<td><code>{{.ClientType}}</code></td>
					<td>
						<i class="fas fa-info-circle" title="The load testing client type used to interact with the Tendermint network"></i>
					</td>
				</tr>

				<tr>
					<td>Slave node count</td>
					<td>{{.SlaveNodeCount}}</td>
					<td>
						<i class="fas fa-info-circle" title="The number of slave nodes used to execute this load test (each responsible for spawning clients)"></i>
					</td>
				</tr>

				<tr>
					<td>Client spawn</td>
					<td>{{.ClientSpawn}}</td>
					<td>
						<i class="fas fa-info-circle" title="The maximum number of clients that were spawned per slave node during this load test"></i>
					</td>
				</tr>

				<tr>
					<td>Client spawn rate</td>
					<td>{{.ClientSpawnRate}}</td>
					<td>
						<i class="fas fa-info-circle" title="The number of clients spawned per second on a single slave node"></i>
					</td>
				</tr>

				<tr>
					<td>Client request wait min</td>
					<td>{{.ClientRequestWaitMin}}</td>
					<td>
						<i class="fas fa-info-circle" title="The minimum time a client waited before firing off each request"></i>
					</td>
				</tr>

				<tr>
					<td>Client request wait max</td>
					<td>{{.ClientRequestWaitMax}}</td>
					<td>
						<i class="fas fa-info-circle" title="The maximum time a client waited before firing off each request"></i>
					</td>
				</tr>

				<tr>
					<td>Client max interactions</td>
					<td>{{.ClientMaxInteractions}}</td>
					<td>
						<i class="fas fa-info-circle" title="The maximum number of interactions between a client and the Tendermint network"></i>
					</td>
				</tr>

				<tr>
					<td>Test run time</td>
					<td>{{.AbsTotalTime}}</td>
					<td>
						<i class="fas fa-info-circle" title="The total time (from the master's perspective) in which the load testing completed"></i>
					</td>
				</tr>
			</tbody>
		</table>
	</div>
	</section>

	<section class="section">
	<div class="container">
			<h2 class="title is-2">Charts</h2>
			<h3 class="subtitle is-3">Interaction Response Times</h3>
			<div class="columns">
				<div class="column is-one-quarter">
					<table class="table is-striped is-hoverable">
						<thead>
							<tr>
								<th>Parameter</th>
								<th>Value</th>
							</tr>
						</thead>
						<tbody>
							<tr>
								<td>Total clients</td>
								<td>{{.InteractionsTotalClients}}</td>
							</tr>
							<tr>
								<td>Interactions/sec overall (absolute)</td>
								<td>{{.InteractionsPerSec}}</td>
							</tr>
							<tr>
								<td>Interaction count</td>
								<td>{{.InteractionsCount}}</td>
							</tr>
							<tr>
								<td>Interaction error count</td>
								<td>{{.InteractionsErrors}}</td>
							</tr>
							<tr>
								<td>Interaction error rate</td>
								<td>{{.InteractionsErrorRate}}</td>
							</tr>
							<tr>
								<td>Interaction min time</td>
								<td>{{.InteractionsMinTime}}</td>
							</tr>
							<tr>
								<td>Interaction max time</td>
								<td>{{.InteractionsMaxTime}}</td>
							</tr>
						</tbody>
					</table>
				</div>
				<div class="column">
					<canvas id="interaction-rt-chart"></canvas>
				</div>
			</div>
	</div>
	</section>

	<section class="section">
		<div class="container">
			<h3 class="subtitle is-3">Request Response Times</h3>

			{{range $i, $req := $.Requests}}
			<h4 class="subtitle is-4"><code>{{$req.Name}}</code></h4>
			<div class="columns">
				<div class="column is-one-quarter">
					<table class="table is-striped is-hoverable">
						<thead>
							<tr>
								<th>Parameter</th>
								<th>Value</th>
							</tr>
						</thead>
						<tbody>
							<tr>
								<td>Requests/sec overall (absolute)</td>
								<td>{{$req.PerSec}}</td>
							</tr>
							<tr>
								<td>Request count</td>
								<td>{{$req.Count}}</td>
							</tr>
							<tr>
								<td>Request error count</td>
								<td>{{$req.Errors}}</td>
							</tr>
							<tr>
								<td>Request error rate</td>
								<td>{{$req.ErrorRate}}</td>
							</tr>
							<tr>
								<td>Request min time</td>
								<td>{{$req.MinTime}}</td>
							</tr>
							<tr>
								<td>Request max time</td>
								<td>{{$req.MaxTime}}</td>
							</tr>
						</tbody>
					</table>
				</div>
				<div class="column">
					<canvas id="{{$req.Name}}-rt-chart"></canvas>
				</div>
			</div>
			{{end}}

		</div>
	</section>

	<script type="text/javascript" src="https://use.fontawesome.com/releases/v5.3.1/js/all.js"></script>
	<script type="text/javascript" src="https://cdn.jsdelivr.net/npm/chart.js@2.8.0/dist/Chart.min.js"></script>

	<script>
		var colorPalette = [
			'hsl(171, 100%, 41%)',
			'hsl(217, 71%, 53%)',
			'hsl(204, 86%, 53%)',
			'hsl(141, 71%, 48%)',
			'hsl(348, 100%, 61%)',
			'hsl(0, 0%, 71%)'
		];
		var curColor = 0;

		function nextColor() {
			result = colorPalette[curColor];
			curColor++;
			if (curColor >= colorPalette.length) {
				curColor = 0;
			}
			return result;
		}

		function generateHistogram(elemID, bins, counts, color, xtitle, ytitle) {
			var ctx = document.getElementById(elemID).getContext('2d');
			var chart = new Chart(ctx, {
				type: 'bar',
				data: {
					labels: bins,
					datasets: [{
						label: ytitle,
						backgroundColor: color,
						data: counts
					}]
				},
				options: {
					scales: {
						xAxes: [{
							display: true,
							scaleLabel: {
								display: true,
								labelString: xtitle,
							}
						}],
						yAxes: [{
							display: true,
							scaleLabel: {
								display: true,
								labelString: ytitle
							},
							ticks: {
								beginAtZero: true
							}
						}]
					}
				}
			});
		}

		generateHistogram(
			"interaction-rt-chart", 
			[
{{.InteractionsResponseTimesBins}}
			], 
			[
{{.InteractionsResponseTimesCounts}}
			], 
			nextColor(), 
			"Interaction Response Time (milliseconds)",
			"Counts"
		);

		{{range $i, $req := $.Requests}}
		generateHistogram(
			"{{$req.Name}}-rt-chart", 
			[
{{$req.ResponseTimesBins}}
			], 
			[
{{$req.ResponseTimesCounts}}
			], 
			nextColor(), 
			"{{$req.Name}} Response Time (milliseconds)",
			"Counts"
		);
		{{end}}
	</script>
</body>
</html>
`

// SingleTestSummaryContext represents context for being able to render a single
// load test's summary plot.
type SingleTestSummaryContext struct {
	// Summary parameters
	ClientType            template.HTML
	SlaveNodeCount        template.HTML
	ClientSpawn           template.HTML
	ClientSpawnRate       template.HTML
	ClientRequestWaitMin  template.HTML
	ClientRequestWaitMax  template.HTML
	ClientRequestTimeout  template.HTML
	ClientMaxInteractions template.HTML
	AbsTotalTime          template.HTML

	// Interaction-related parameters
	InteractionsTotalClients template.HTML
	InteractionsPerSec       template.HTML
	InteractionsCount        template.HTML
	InteractionsErrors       template.HTML
	InteractionsErrorRate    template.HTML
	InteractionsMinTime      template.HTML
	InteractionsMaxTime      template.HTML

	// For the interaction response times histogram
	InteractionsResponseTimesBins   template.JS
	InteractionsResponseTimesCounts template.JS

	// Request-related parameters
	Requests []SingleTestSummaryRequestParams
}

// SingleTestSummaryRequestParams encapsulates parameters for a single request
// type in the above plot.
type SingleTestSummaryRequestParams struct {
	Name      template.HTML
	PerSec    template.HTML
	Count     template.HTML
	Errors    template.HTML
	ErrorRate template.HTML
	MinTime   template.HTML
	MaxTime   template.HTML

	// For the request response times histogram
	ResponseTimesBins   template.JS
	ResponseTimesCounts template.JS
}

// NewSingleTestSummaryContext creates the relevant context to be able to render
// the single load test plot.
func NewSingleTestSummaryContext(cfg *Config, stats *messages.CombinedStats) SingleTestSummaryContext {
	icstats := CalculateStats(stats.Interactions)
	// flatten the interaction response time histogram
	ibins, icounts := FlattenResponseTimeHistogram(stats.Interactions.ResponseTimes, "				")
	return SingleTestSummaryContext{
		ClientType:            template.HTML(cfg.Clients.Type),
		SlaveNodeCount:        template.HTML(fmt.Sprintf("%d", cfg.Master.ExpectSlaves)),
		ClientSpawn:           template.HTML(fmt.Sprintf("%d", cfg.Clients.Spawn)),
		ClientSpawnRate:       template.HTML(fmt.Sprintf("%.1f", cfg.Clients.SpawnRate)),
		ClientRequestWaitMin:  template.HTML(fmt.Sprintf("%.0fms", float64(time.Duration(cfg.Clients.RequestWaitMin))/float64(time.Millisecond))),
		ClientRequestWaitMax:  template.HTML(fmt.Sprintf("%.0fms", float64(time.Duration(cfg.Clients.RequestWaitMax))/float64(time.Millisecond))),
		ClientRequestTimeout:  template.HTML(fmt.Sprintf("%.0fms", float64(time.Duration(cfg.Clients.RequestTimeout))/float64(time.Millisecond))),
		ClientMaxInteractions: template.HTML(fmt.Sprintf("%d", cfg.Clients.MaxInteractions)),
		AbsTotalTime:          template.HTML(fmt.Sprintf("%.2fs", float64(stats.Interactions.AbsTotalTime)/float64(time.Second))),

		InteractionsTotalClients: template.HTML(fmt.Sprintf("%d", stats.Interactions.TotalClients)),
		InteractionsPerSec:       template.HTML(fmt.Sprintf("%.2f", icstats.AbsPerSec)),
		InteractionsCount:        template.HTML(fmt.Sprintf("%d", stats.Interactions.Count)),
		InteractionsErrors:       template.HTML(fmt.Sprintf("%d", stats.Interactions.Errors)),
		InteractionsErrorRate:    template.HTML(fmt.Sprintf("%.2f", icstats.FailureRate)),
		InteractionsMinTime:      template.HTML(fmt.Sprintf("%.1fms", float64(time.Duration(stats.Interactions.MinTime))/float64(time.Millisecond))),
		InteractionsMaxTime:      template.HTML(fmt.Sprintf("%.1fms", float64(time.Duration(stats.Interactions.MaxTime))/float64(time.Millisecond))),

		InteractionsResponseTimesBins:   template.JS(ibins),
		InteractionsResponseTimesCounts: template.JS(icounts),

		Requests: buildRequestsCtx(stats.Requests),
	}
}

func buildRequestsCtx(stats map[string]*messages.SummaryStats) []SingleTestSummaryRequestParams {
	result := make([]SingleTestSummaryRequestParams, 0)
	reqNames := make([]string, 0)
	rcstats := make(map[string]*CalculatedStats)
	for reqName, reqStats := range stats {
		reqNames = append(reqNames, reqName)
		rcstats[reqName] = CalculateStats(reqStats)
	}
	// now sort the request names alphabetically
	sort.SliceStable(reqNames[:], func(i, j int) bool {
		return strings.Compare(reqNames[i], reqNames[j]) < 0
	})
	for _, reqName := range reqNames {
		rbins, rcounts := FlattenResponseTimeHistogram(stats[reqName].ResponseTimes, "				")
		params := SingleTestSummaryRequestParams{
			Name:                template.HTML(reqName),
			PerSec:              template.HTML(fmt.Sprintf("%.2f", rcstats[reqName].AbsPerSec)),
			Count:               template.HTML(fmt.Sprintf("%d", stats[reqName].Count)),
			Errors:              template.HTML(fmt.Sprintf("%d", stats[reqName].Errors)),
			ErrorRate:           template.HTML(fmt.Sprintf("%.2f", rcstats[reqName].FailureRate)),
			MinTime:             template.HTML(fmt.Sprintf("%.1fms", float64(stats[reqName].MinTime)/float64(time.Millisecond))),
			MaxTime:             template.HTML(fmt.Sprintf("%.1fms", float64(stats[reqName].MaxTime)/float64(time.Millisecond))),
			ResponseTimesBins:   template.JS(rbins),
			ResponseTimesCounts: template.JS(rcounts),
		}
		result = append(result, params)
	}
	return result
}

// RenderSingleTestSummaryPlot is a convenience method to render the single test
// summary plot to a string, ready to be written to an output file.
func RenderSingleTestSummaryPlot(cfg *Config, stats *messages.CombinedStats) (string, error) {
	ctx := NewSingleTestSummaryContext(cfg, stats)
	tmpl, err := template.New("single-test-summary-plot").Parse(SingleTestSummaryPlot)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err = tmpl.Execute(&b, ctx); err != nil {
		return "", err
	}
	return b.String(), nil
}
