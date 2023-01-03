import pytest
import os
import logging

from ..common import check_events_from_splunk

@pytest.mark.parametrize("test_key, test_value, expected", [
    ("object.kind", "event", 1),
    ("kind", "pod", 1),
    ("kind", "namespace", 1),
    ("kind", "node", 1)
])
def test_k8s_objects(setup, test_key, test_value, expected):
    '''
    Test that user specified index can successfully index the
    objects stream from k8s.
    '''
    logging.getLogger().info("testing test_splunk_index input={0} \
                 expected={1} event(s)".format(test_value, expected))
    index_objects = os.environ.get("CI_INDEX_EVENTS", "ci_events")

    search_query = f'index={index_objects} {test_key}={test_value}'
    events = check_events_from_splunk(start_time="-1h@h",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(search_query)],
                                      password=setup["splunk_password"])
    logging.getLogger().info("Splunk received %s events in the last minute",
                             len(events))
    assert len(events) >= expected

@pytest.mark.parametrize("test_key, test_value, expected", [
    ("sourcetype", "kube:object:Pod", 1),
    ("sourcetype", "kube:object:namespace", 1),
    ("sourcetype", "kube:object:node", 1)
])
def test_k8s_objects_sourcetype(setup, test_key, test_value, expected):
    '''
    Test that known k8s objects sourcetypes are present in target index
    '''
    logging.getLogger().info("testing test_splunk_index input={0} \
                 expected={1} event(s)".format(test_value, expected))
    index_objects = os.environ.get("CI_INDEX_EVENTS", "ci_events")

    search_query = f'index={index_objects} {test_key}={test_value}'
    events = check_events_from_splunk(start_time="-1h@h",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(search_query)],
                                      password=setup["splunk_password"])
    logging.getLogger().info("Splunk received %s events in the last minute",
                             len(events))
    assert len(events) >= expected

