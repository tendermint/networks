"""
Locust-based load testing application for generating load on a Tendermint
network running the `kvstore` proxy application, but over the WebSockets RPC
API.
"""

import binascii
import os
import json
import time
import base64
import websocket

from locust import Locust, TaskSequence, events, seq_task


MIN_WAIT = int(os.environ.get('MIN_WAIT', 100))
MAX_WAIT = int(os.environ.get('MAX_WAIT', 500))
DEFAULT_HOSTS = [
    'tik0.sredev.co:26657',
    'tik1.sredev.co:26657',
    'tik2.sredev.co:26657',
    'tik3.sredev.co:26657',
]
# Parse the nodes we want to hit from the environment
HOST_URLS = os.environ.get('HOST_URLS', '::'.join(DEFAULT_HOSTS)).split('::')


class WebSocketsKVStoreClient(object):

    client = None

    def __init__(self, host):
        self.host = host
        self.client = websocket.create_connection(host)

    def _wrap_call(self, rpc_obj, expected=None):
        start_time = time.time()
        result = None
        try:
            rpc_obj["id"] = binascii.hexlify(os.urandom(16)).decode('utf-8')
            data = json.dumps(rpc_obj)
            self.client.send(data)
            result = json.loads(self.client.recv())
        except Exception as e:
            total_time = int((time.time() - start_time) * 1000.0)
            events.request_failure.fire(request_type="ws", name=rpc_obj['method'], response_time=total_time, exception=e)
        else:
            total_time = int((time.time() - start_time) * 1000.0)
            result_value = result.get('result', {}).get('response', {}).get('value', None)
            if expected is not None and result_value is not None:
                result_value_decoded = base64.b64decode(result_value).decode('utf-8')
                if result_value_decoded != expected:
                    events.request_failure.fire(request_type='ws', name=rpc_obj['method'], response_time=total_time, exception=Exception("Response value mismatch"))
                    return result
            elif result_value is not None:
                events.request_failure.fire(request_type="ws", name=rpc_obj['method'], response_time=total_time, exception=Exception("Unexpected response format from server"))
            events.request_success.fire(request_type="ws", name=rpc_obj['method'], response_time=total_time, response_length=len(result))
        return result

    def put(self, k, v):
        return self._wrap_call({
            "jsonrpc": "2.0",
            "method": "broadcast_tx_sync",
            "params": {
                "tx": base64.b64encode(("%s=%s" % (k, v)).encode('utf-8')).decode('utf-8'),
            },
        })

    def get(self, k, expected=None):
        return self._wrap_call({
            "jsonrpc": "2.0",
            "method": "abci_query",
            "params": {
                "path": "",
                "data": binascii.hexlify(k.encode('utf-8')).decode('utf-8'),
                "height": "0",
                "prove": False,
            },
        }, expected=expected)

    def close(self):
        print("Closing client for %s" % self.host)
        self.client.close()


class WebSocketsLocust(Locust):
    """Allows us to interact with hosts via WebSockets instead of HTTP."""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.client = WebSocketsKVStoreClient(self.host)
    
    def teardown(self):
        self.client.close()


class KVStoreBalancedRWTaskSequence(TaskSequence):
    """A balanced read/write task set for interacting with the nodes."""

    @seq_task(1)
    def put_value(self):
        self.key = binascii.hexlify(os.urandom(16)).decode('utf-8')
        self.value = binascii.hexlify(os.urandom(16)).decode('utf-8')
        self.client.put(self.key, self.value)

    @seq_task(2)
    def get_value(self):
        self.client.get(self.key, expected=self.value)


def make_node_locust_class(host_url, node_id):
    class NodeUserLocust(WebSocketsLocust):
        host = host_url
        weight = 1
        task_set = KVStoreBalancedRWTaskSequence
        min_wait = MIN_WAIT
        max_wait = MAX_WAIT

    NodeUserLocust.__name__ = "Node%dUserLocust" % node_id
    return NodeUserLocust


for i in range(len(HOST_URLS)):
    node_cls = make_node_locust_class('ws://%s/websocket' % HOST_URLS[i], i)
    # Inject the class into global space, so it will be picked up by Locust
    globals()[node_cls.__name__] = node_cls
    print("Created class %s" % node_cls.__name__)
