#!/usr/bin/env python3

import csv
import os
import os.path
import sys

import numpy as np
import matplotlib.pyplot as plt


LOADTEST_MASTER = os.environ.get('LOADTEST_MASTER', 'tok0')

# We expect that the following environment variables MUST be set
START_NUM_CLIENTS = int(os.environ['START_NUM_CLIENTS'])
END_NUM_CLIENTS = int(os.environ['END_NUM_CLIENTS'])
INC_NUM_CLIENTS = int(os.environ['INC_NUM_CLIENTS'])
START_HATCH_RATE = int(os.environ['START_HATCH_RATE'])
END_HATCH_RATE = int(os.environ['END_HATCH_RATE'])
INC_HATCH_RATE = int(os.environ['INC_HATCH_RATE'])
RUN_TIME = os.environ['RUN_TIME']
RESULTS_OUTPUT_DIR = os.environ['RESULTS_OUTPUT_DIR']
PLOT_RESULTS_DIR = os.environ['PLOT_RESULTS_DIR']


class LoadTestResults:
    """Results from a single load test out of the collection."""

    def __init__(self, num_clients, hatch_rate, requests_results, dist_results):
        self.num_clients = num_clients
        self.hatch_rate = hatch_rate
        self.requests_results = requests_results
        self.dist_results = dist_results


class RequestsResults:
    def __init__(self, request_count, failure_count, median_rt, avg_rt, min_rt, max_rt, requests_per_sec):
        self.request_count = request_count
        self.failure_count = failure_count
        self.median_rt = median_rt
        self.avg_rt = avg_rt
        self.min_rt = min_rt
        self.max_rt = max_rt
        self.requests_per_sec = requests_per_sec


def iterate_results():
    num_clients = START_NUM_CLIENTS
    hatch_rate = START_HATCH_RATE
    while num_clients <= END_NUM_CLIENTS:
        results_path = os.path.join(
            RESULTS_OUTPUT_DIR,
            "%d-clients-%s" % (num_clients, RUN_TIME),
        )
        yield num_clients, hatch_rate, results_path
        num_clients += INC_NUM_CLIENTS
        hatch_rate += INC_HATCH_RATE


def check_all_results_present():
    """Does a quick check to make sure all results files are present and
    accounted for."""
    success = True
    for _, _, results_path in iterate_results():
        if not os.path.exists(results_path):
            print("ERROR: Missing results path: %s" % results_path)
            success = False

        host_path = os.path.join(results_path, LOADTEST_MASTER)
        if not os.path.exists(host_path):
            print("ERROR: Missing results host path: %s" % host_path)
            success = False
        
        for csv_file in ['loadtest_distribution.csv', 'loadtest_requests.csv']:
            csv_file_path = os.path.join(host_path, csv_file)
            if not os.path.exists(csv_file_path):
                print("ERROR: Missing results file: %s" % csv_file_path)
                success = False

    return success


def load_results():
    """Loads the results from the load testing CSV files."""
    results = []
    for num_clients, hatch_rate, results_path in iterate_results():
        host_path = os.path.join(results_path, LOADTEST_MASTER)

        requests_results = None
        dist_results = None

        with open(os.path.join(host_path, 'loadtest_requests.csv'), 'r') as f:
            reader = csv.DictReader(f)
            for row in reader:
                if row['Name'] == 'Total':
                    requests_results = RequestsResults(
                        int(row['# requests']),
                        int(row['# failures']),
                        int(row['Median response time']),
                        float(row['Average response time']),
                        int(row['Min response time']),
                        int(row['Max response time']),
                        float(row['Requests/s']),
                    )
        
        if not requests_results:
            print("ERROR: Failed to load requests results for host %s, num_clients=%d" % (LOADTEST_MASTER, num_clients))
            sys.exit(2)
        
        results.append(LoadTestResults(
            num_clients,
            hatch_rate,
            requests_results,
            dist_results,
        ))
    
    return results


def plot_response_times(results):
    """Plots requests' response times on a bar chart."""
    median_rt, avg_rt, min_rt, max_rt, num_clients = [], [], [], [], []
    for result in results:
        num_clients.append(result.num_clients)
        min_rt.append(result.requests_results.min_rt)
        median_rt.append(result.requests_results.median_rt)
        avg_rt.append(result.requests_results.avg_rt)
        max_rt.append(result.requests_results.max_rt)

    plt.scatter(num_clients, min_rt, marker='v', color='#0000aa', label='min')
    plt.scatter(num_clients, median_rt, marker='o', color='#009900', label='median')
    plt.scatter(num_clients, avg_rt, marker='>', color='#00dd00', label='average')
    plt.scatter(num_clients, max_rt, marker='^', color='#aa0000', label='max')

    plt.ylabel('Response time (ms)')
    plt.title('Response times by number of clients')
    plt.xlabel('Number of clients')
    plt.legend()

    plt.grid(True)
    output_file = os.path.join(PLOT_RESULTS_DIR, 'response_times.png')
    plt.savefig(output_file, format='png')
    print("Wrote response times vs number of clients to file: %s" % output_file)


def plot_request_rate(results):
    """Plots request rate and % failure rate versus number of clients."""
    reqs_per_sec, failures, num_clients = [], [], []
    for result in results:
        num_clients.append(result.num_clients)
        reqs_per_sec.append(result.requests_results.requests_per_sec)
        failure_rate = 100.0 * (float(result.requests_results.failure_count) / float(result.requests_results.request_count))
        failures.append(failure_rate)
    
    fig, ax1 = plt.subplots()
    
    ax1.set_xlabel('Number of clients')
    ax1.set_ylabel('Requests/sec')
    ax1.plot(num_clients, reqs_per_sec, color='#0000aa', marker='o', label='requests/sec')
    ax1.tick_params(axis='y', labelcolor='#0000aa')

    ax2 = ax1.twinx()

    ax2.set_ylabel('Request failure rate (%)')
    ax2.plot(num_clients, failures, marker='v', color='#aa0000', label='failure %')
    ax2.tick_params(axis='y', labelcolor='#aa0000')

    plt.title('Request and failure rate by number of clients')

    fig.tight_layout()
    
    output_file = os.path.join(PLOT_RESULTS_DIR, 'request_rate.png')
    plt.savefig(output_file, format='png')
    print("Wrote request and failure rate vs number of clients to file: %s" % output_file)


def main():
    if not check_all_results_present():
        sys.exit(1)
    
    results = load_results()
    plot_response_times(results)
    plot_request_rate(results)


if __name__ == "__main__":
    main()
