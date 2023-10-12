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


@pytest.fixture(scope="module", autouse=True)
def setup_for_agent_tests():
    # Set up code before the test
    logger.info("Setup: prepare env before agent tests")
    # currently tests are setting their own collector configuration

    # Yield control to the test
    yield

    # Teardown code after the test
    logger.info("Teardown: clean up after agent tests")
    default_yaml_file = "./../ci_scripts/sck_otel_values.yaml"
    yaml_fields_recall = {
        "splunkPlatform.index": os.environ.get("CI_INDEX_EVENTS"),
        "splunkPlatform.metricsIndex": os.environ.get("CI_INDEX_METRICS"),
        "splunkPlatform.token": os.environ.get("CI_SPLUNK_HEC_TOKEN"),
        "splunkPlatform.endpoint": "https://"
        + os.environ.get("CI_SPLUNK_HOST")
        + ":8088/services\/collector",
    }
    k8s_helper.upgrade_helm(default_yaml_file, yaml_fields_recall)


def test_all_agent_logs_correctly_ingested_into_splunk(setup):
    """
    Test that agent logs are correctly ingested into Splunk
    """
    logger.info("testing that agent logs are correctly ingested into Splunk")
    # prepare connector for test
    yaml_file = AGENT_VALUES_YAML
    yaml_fields = {
        "splunkPlatform.index": INDEX_MAIN,
        "splunkPlatform.token": os.environ.get("CI_SPLUNK_HEC_TOKEN"),
        "splunkPlatform.endpoint": "https://"
        + os.environ.get("CI_SPLUNK_HOST")
        + ":8088/services/collector",
    }
    k8s_helper.upgrade_helm(yaml_file, yaml_fields)

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
        start_time="-5m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    assert len(events) >= 1  # ensure that we are getting logs
    agent_logs = k8s_helper.get_pod_logs(full_pod_name)
    match_counter = 0
    for event in events:
        for line in agent_logs:
            if event["_raw"].strip() == line.strip():
                match_counter += 1
                break
    assert len(events) == match_counter


def test_no_agent_logs_ingested_into_splunk_with_exclude_agent_logs_flag(setup):
    """
    Test that agent logs are not ingested into Splunk while exclude agent logs flag is set
    """
    logger.info(
        "Testing that that agent logs are not ingested into Splunk while exclude agent logs flag is set"
    )
    # prepare connector for test
    yaml_file = AGENT_VALUES_YAML
    # Open the YAML file for reading
    with open(yaml_file, "r") as file:
        data = yaml.safe_load(file)  # Parse the YAML data

    # Modify data
    data["logsCollection"]["containers"]["excludeAgentLogs"] = True

    # write YAML file
    new_yaml = "exclude_agent_logs.yaml"
    with open(new_yaml, "w") as file:
        yaml.safe_dump(data, file)

    yaml_fields = {
        "splunkPlatform.index": INDEX_MAIN,
        "splunkPlatform.token": os.environ.get("CI_SPLUNK_HEC_TOKEN"),
        "splunkPlatform.endpoint": "https://"
        + os.environ.get("CI_SPLUNK_HOST")
        + ":8088/services/collector",
    }
    k8s_helper.upgrade_helm(new_yaml, yaml_fields)
    time.sleep(10)  # wait for some time to have more time for potential logs ingestion

    search_query = (
        "index="
        + INDEX_MAIN
        + " k8s.pod.name="
        + k8s_helper.get_pod_full_name("agent")
        + " source=*/otel-collector/*.log"
    )
    logger.info(f"Query: {search_query}")
    events = check_events_from_splunk(
        start_time="-5m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the 5 minutes", len(events))
    assert len(events) == 0  # ensure that we are not getting any logs

@pytest.mark.parametrize("persistence_enabled", [
    "true",
    "false",
])
def test_agent_logs_with_persistent_storage(setup, persistence_enabled):
        yaml_file = AGENT_VALUES_YAML
        yaml_fields = {
            "splunkPlatform.index": INDEX_MAIN,
            "splunkPlatform.token": os.environ.get("CI_SPLUNK_HEC_TOKEN"),
            "splunkPlatform.endpoint": "http://" # incorrect protocol, to trigger non-permanent retryable error
            + os.environ.get("CI_SPLUNK_HOST")
            + ":8088/services/collector",
            "splunkPlatform.sendingQueue.persistentQueue.enabled": persistence_enabled,
        }
        k8s_helper.upgrade_helm(yaml_file, yaml_fields)
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
            start_time="-5m@m",
            url=setup["splunkd_url"],
            user=setup["splunk_user"],
            query=["search {0}".format(search_query)],
            password=setup["splunk_password"],
        )
        logger.info("Splunk received %s events in the last minute", len(events))
        assert len(events) == 0 # shouldn't receive any events, incorrect protocol!!

        yaml_fields["splunkPlatform.endpoint"] = "https://" \
            + os.environ.get("CI_SPLUNK_HOST") \
            + ":8088/services/collector"  # correct protocol
        k8s_helper.upgrade_helm(yaml_file, yaml_fields)
        time.sleep(5)  # wait for some time to have more time for potential logs ingestion

        # check logs from previous pod
        events = check_events_from_splunk(
            start_time="-5m@m",
            url=setup["splunkd_url"],
            user=setup["splunk_user"],
            query=["search {0}".format(search_query)],
            password=setup["splunk_password"],
        )
        logger.info("Splunk received %s events in the last minute", len(events))
        assert len(events) >= 1 if persistence_enabled == "true" else len(events) == 0
