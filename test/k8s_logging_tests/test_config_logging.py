import pytest
import time
import os
import logging
import json
import sys
from urllib.parse import urlparse
from ..common import check_events_from_splunk, create_index_in_splunk, delete_index_in_splunk

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)
formatter = logging.Formatter('%(message)s')
handler = logging.StreamHandler(sys.stdout)
handler.setFormatter(formatter)
logger.addHandler(handler)

@pytest.mark.parametrize("label,index,expected", [
    ("pod-w-index-wo-ns-index", "pod-anno", 1),
    ("pod-wo-index-w-ns-index", "ns-anno", 1),
    ("pod-w-index-w-ns-index", "pod-anno", 1)
])
def test_label_collection(setup, label, index, expected):
    '''
    Test that user specified labels is attached as a metadata to all the logs
    '''
    logger.info("testing label_app label={0} index={1} expected={2} event(s)".format(
        label, index, expected))
    search_query = "index=" + index + " k8s.pod.labels.app::" + label
    events = check_events_from_splunk(start_time="-1h@h",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(
                                          search_query)],
                                      password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) >= expected

@pytest.mark.parametrize("label,index,value,expected", [
    ("pod-w-index-wo-ns-index", "pod-anno", "pod-value-2", 1),
    # ("pod-wo-index-w-ns-index", "ns-anno", "ns-value", 1),
    ("pod-w-index-w-ns-index", "pod-anno", "pod-value-1", 1)
])
def test_custom_metadata_fields_annotations(setup, label, index, value, expected):

    '''
    Test that user specified labels are resolved from the user specified annotations and attached as a metadata
    to all the logs
    '''
    logger.info("testing custom metadata annotation label={0} value={1} expected={2} event(s)".format(
        label, value, expected))
    search_query = "index=" + index + " k8s.pod.labels.app::" + label + " customField::" + value

    events = check_events_from_splunk(start_time="-1h@h",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(
                                          search_query)],
                                      password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) >= expected

@pytest.mark.parametrize("index,expected", [
    ("test_metrics", 1)
])
def test_metric_index_from_annotations(setup, index, expected):

    '''
    Test that metrics are being sent to "test_metrics" index, as defined by splunk.com/metricsIndex annotation added during setup
    '''
    logger.info("testing for metrics index={0} expected={1} event(s)".format(index, expected))
    search_query = "index=" + index

    events = check_events_from_splunk(start_time="-1h@h",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["mpreview {0}".format(
                                          search_query)],
                                      password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) >= expected

@pytest.mark.parametrize("index,sourcetype,expected", [
    ("test_metrics", "sourcetype-anno", 1)
])
def test_metric_sourcetype_from_annotations(setup, index, sourcetype, expected):

    '''
    Test that metrics are being assigned the "sourcetype-anno" sourcetype, as defined by splunk.com/sourcetype annotation added during setup
    '''
    logger.info("testing for metrics index={0} sourcetype={1} expected={2} event(s)".format(index, sourcetype, expected))
    search_query = "index={0} filter=\"sourcetype={1}\"".format(index, sourcetype)

    events = check_events_from_splunk(start_time="-1h@h",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["mpreview {0}".format(
                                          search_query)],
                                      password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) >= expected