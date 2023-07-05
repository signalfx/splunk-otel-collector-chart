import re

import pytest
import logging
import sys
from common import check_events_from_splunk
from k8s_agent_pod_tests import k8s_helper

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)
formatter = logging.Formatter("%(message)s")
handler = logging.StreamHandler(sys.stdout)
handler.setFormatter(formatter)
logger.addHandler(handler)


@pytest.fixture(scope="module", autouse=True)
def setup_for_agent_tests():
    # Set up code before the test
    print("Setup: prepare env before agent tests")
    # module_yaml_file = "k8s_correctness_tests/local_config/agent_test_values.yaml"
    module_yaml_file = "local_config/agent_test_values.yaml"
    k8s_helper.upgrade_helm(module_yaml_file)

    # Yield control to the test
    yield

    # Teardown code after the test
    print("Teardown: clean up after agent tests")
    default_yaml_file = "../../ci_scripts/sck_otel_values_new.yaml"
    k8s_helper.upgrade_helm(default_yaml_file)


def test_agent_logs_metadata(setup):
    """
    Test that agent logs have correct metadata:
    - source
    - sourcetype
    - index

    """
    full_pod_name = k8s_helper.get_pod_full_name("agent")

    index_logging = "main"
    search_query = (
        "index="
        + index_logging
        + " k8s.pod.name="
        + full_pod_name
        + ' "Everything is ready. Begin running and processing data."'
    )
    print(f"query: {search_query}")
    events = check_events_from_splunk(
        start_time="-2h@h",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    assert len(events) == 1
    event = events[0]
    print(event)
    sourcetype = "kube:container:otel-collector"
    sorce_regex_part = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
    source_pattern = (
        r"^/var/log/pods/default_"
        + full_pod_name
        + "_"
        + sorce_regex_part
        + "/otel-collector/0.log$"
    )
    assert index_logging == event["index"]
    assert full_pod_name == event["k8s.pod.name"]
    assert sourcetype == event["_sourcetype"]
    assert re.match(
        source_pattern, event["source"]
    ), f"Source does not match the pattern {source_pattern}"


# @pytest.mark.skip("skipping test case execution")
def test_all_agent_logs_correctly_ingested_into_splunk(setup):
    """
    Test that agent logs are correctly ingested into Splunk
    """
    logger.info("testing that agent logs are correctly ingested into Splunk")
    # index_logging = os.environ["CI_INDEX_EVENTS"] if os.environ["CI_INDEX_EVENTS"] else "ci_events"
    # search_query = "index=" + index_logging

    full_pod_name = k8s_helper.get_pod_full_name("agent")

    index_logging = "main"
    search_query = (
        "index="
        + index_logging
        + " k8s.pod.name="
        + full_pod_name
        + " source=*/otel-collector/*.log"
    )
    events = check_events_from_splunk(
        start_time="-2h@h",
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
        # print(event["_raw"])
        for line in agent_logs:
            # Do something with the line
            # print(line.strip())
            if event["_raw"] == line:
                # print("match")
                match_counter += 1
                break
    assert len(events) == match_counter
