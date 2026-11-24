"""Run from the repository root with: python3 functional_tests/profiling_metrics/verify.py /path/to/otelcol.

Requires Helm with the chart dependencies installed, yq, and the chart's Collector build.
"""
import json
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time
from urllib.error import URLError
from urllib.request import Request, urlopen


IDENTITY = {
    "k8s.pod.uid": "same-pod",
    "k8s.container.name": "app",
    "k8s.container.restart_count": "0",
}
CASES = {
    # The pod, container name, and count match before and after a kubelet state loss.
    # A buffered point from the old run keeps its ID even when the new run reuses zero.
    "replacement": ("new-id", None, "new-id", None),
    "delayed-before-reset": ("old-id", None, "old-id", None),
    "unknown-run-with-reset-count": (None, None, None, None),
    "conflicting-point-id": ("new-id", "conflict", "new-id", None),
    "duplicate-point-id": ("new-id", "new-id", "new-id", None),
    "point-only-id": (None, "point-id", None, "point-id"),
    "empty-resource-with-point-id": ("", "point-id", None, "point-id"),
    "empty-point-id": (None, "", None, None),
}


def attributes(values):
    return [{"key": key, "value": {"stringValue": value}}
            for key, value in values.items() if value is not None]


def input_metrics():
    return {"resourceMetrics": [
        {"resource": {"attributes": attributes({**IDENTITY, "test.case": name, "container.id": resource_id})},
         "scopeMetrics": [{"metrics": [{"name": "jvm.memory.used", "gauge": {"dataPoints": [
             {"asInt": "1", "timeUnixNano": str(time.time_ns()), "attributes": attributes({"container.id": point_id})}
         ]}}]}]}
        for name, (resource_id, point_id, _, _) in CASES.items()
    ]}


def rendered_config(mode):
    rendered = subprocess.check_output([
        "helm", "template", "test", "helm-charts/splunk-otel-collector",
        "-f", "unittests/values/basic.yaml", "--set", f"gateway.enabled={str(mode == 'gateway').lower()}",
        "--set", "splunkObservability.profilingEnabled=true",
        "--set", "splunkObservability.profilingMetricsContainerIdEnabled=true",
        "--show-only", f"templates/configmap-{mode}.yaml",
    ])
    return json.loads(subprocess.check_output(["yq", "-o=json", ".data.relay | from_yaml"], input=rendered))


def check(mode, collector, directory):
    rendered = rendered_config(mode)
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]

    output = directory / f"{mode}.jsonl"
    intake = rendered["service"]["pipelines"]["metrics/profiling_ingest"]
    destination = "forward/profiling_metrics"
    assert destination in rendered["service"]["pipelines"]["metrics"]["receivers"]
    config = directory / f"{mode}.json"
    config.write_text(json.dumps({
        "receivers": {"otlp": {"protocols": {"http": {"endpoint": f"127.0.0.1:{port}"}}}},
        "processors": {name: rendered["processors"][name] for name in intake["processors"]},
        "exporters": {"file": {"path": str(output), "flush_interval": "100ms"}},
        "connectors": {destination: rendered["connectors"][destination]},
        "service": {"pipelines": {
            "metrics/profiling_ingest": intake,
            "metrics": {"receivers": [destination], "exporters": ["file"]},
        }},
    }))
    request = Request(
        f"http://127.0.0.1:{port}/v1/metrics",
        data=json.dumps(input_metrics()).encode(),
        headers={"Content-Type": "application/json"},
    )
    log = directory / f"{mode}.log"
    with log.open("w") as stream, subprocess.Popen(
        [collector, "--config", str(config)], stdout=stream, stderr=subprocess.STDOUT,
    ) as process:
        try:
            for _ in range(100):
                if process.poll() is not None:
                    raise RuntimeError(log.read_text())
                try:
                    with urlopen(request, timeout=1) as response:
                        assert response.status == 200
                    break
                except URLError:
                    time.sleep(0.1)
            else:
                raise RuntimeError(log.read_text())
        finally:
            if process.poll() is None:
                process.terminate()

    def read_attributes(obj):
        return {attr["key"]: attr["value"].get("stringValue") for attr in obj.get("attributes", [])}

    received = {}
    for line in output.read_text().splitlines():
        for resource in json.loads(line)["resourceMetrics"]:
            resource_attributes = read_attributes(resource["resource"])
            point = resource["scopeMetrics"][0]["metrics"][0]["gauge"]["dataPoints"][0]
            name = resource_attributes["test.case"]
            assert name not in received, (mode, name)
            received[name] = (resource_attributes.get("container.id"), read_attributes(point).get("container.id"))
    assert received == {name: expected[2:] for name, expected in CASES.items()}, (mode, received)
    print(f"{mode}: {len(received)} container ID cases passed")


if __name__ == "__main__":
    collector = str(Path(sys.argv[1]).resolve())
    with tempfile.TemporaryDirectory() as path:
        for mode in ("agent", "gateway"):
            check(mode, collector, Path(path))
