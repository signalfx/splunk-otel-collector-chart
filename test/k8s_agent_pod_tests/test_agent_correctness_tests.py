import os
import re
import time

import pytest
import logging
import sys

import yaml

from ..common import check_events_from_splunk
from k8s_agent_pod_tests import k8s_helper

AGENT_VALUES_YAML = "config_yaml_files/agent_tests_values.yaml"

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)
formatter = logging.Formatter("%(message)s")
handler = logging.StreamHandler(sys.stdout)
handler.setFormatter(formatter)
logger.addHandler(handler)


INDEX_MAIN = "main"


def test_agent_logs_metadata(setup):
    """
    Test that agent logs have correct metadata:
    - source
    - sourcetype
    - index

    """
    full_pod_name = k8s_helper.get_pod_full_name("agent")
    search_query = (
        "index="
        + INDEX_MAIN
        + " k8s.pod.name="
        + full_pod_name
        + ' "Everything is ready. Begin running and processing data."'
    )
    logger.info(f"Query: {search_query}")
    events = check_events_from_splunk(
        start_time="-1h@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events", len(events))
    assert len(events) >= 1
    event = events[0]
    sourcetype = "kube:container:otel-collector"
    sorce_regex_part = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
    source_pattern = (
        r"^/var/log/pods/default_"
        + full_pod_name
        + "_"
        + sorce_regex_part
        + "/otel-collector/0.log$"
    )
    assert INDEX_MAIN == event["index"]
    assert full_pod_name == event["k8s.pod.name"]
    assert sourcetype == event["_sourcetype"]
    assert re.match(
        source_pattern, event["source"]
    ), f"Source does not match the pattern {source_pattern}"


def test_all_agent_logs_correctly_ingested_into_splunk(setup):
    """
    Test that agent logs are correctly ingested into Splunk
    """
    logger.info("testing that agent logs are correctly ingested into Splunk")
    full_pod_name = k8s_helper.get_pod_full_name("agent")
    search_query = (
        "index="
        + INDEX_MAIN
        + " k8s.pod.name="
        + full_pod_name
        + " source=*/otel-collector/*.log"
    )
    logger.info(f"Query: {search_query}")
    events = check_events_from_splunk(
        start_time="-1h@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events", len(events))
    assert len(events) >= 1  # ensure that we are getting logs
    agent_logs = k8s_helper.get_pod_logs(full_pod_name)
    match_counter = 0
    for event in events:
        for line in agent_logs:
            if event["_raw"].strip() == line.strip():
                match_counter += 1
                break
    assert len(events) == match_counter
