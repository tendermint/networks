# Tendermint v0.28.0 Load Testing

This folder contains experimental results using the following parameters.

| Parameter | Description/Value |
| --- | --- |
| Tendermint version | v0.29.1 |
| Proxy app | `kvstore` |
| Network configuration | `002-no-empty-blocks-issue` |
| Tendermint nodes | 4 |
| Load test nodes | 4 |
| Protocol | WebSockets |
| RPCs used | `broadcast_tx_sync`, `abci_query` |
| Starting clients | 100 |
| Ending clients | 3000 |
| Execution time | 10 min per load |
| Redeploy network between tests | Yes |

## Results

![Request and failure rate versus number of clients](./plots/request_rate.png)

![Response times versus number of clients](./plots/response_times.png)
