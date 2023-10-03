import pytest
import os
import logging

from ..common import check_events_from_splunk

def test_traces(setup):
    '''
    Test that traces are received by Splunk.
    '''
    logging.getLogger().info("testing test_traces")
    index_traces = os.environ.get("CI_INDEX_TRACES", "ci_traces")

    tomcat_search_query = f'index={index_traces} index=traces name="Render /index.jsp"'
    events = check_events_from_splunk(start_time="-1h@h",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(tomcat_search_query)],
                                      password=setup["splunk_password"])
    logging.getLogger().info("Splunk received %s events in the last minute",
                             len(events))
    assert len(events) >= 0

    nodejs_search_query = f'index={index_traces} index=traces name="GET /"'
    events = check_events_from_splunk(start_time="-1h@h",
                                  url=setup["splunkd_url"],
                                  user=setup["splunk_user"],
                                  query=["search {0}".format(nodejs_search_query)],
                                  password=setup["splunk_password"])
    logging.getLogger().info("Splunk received %s events in the last minute",
                         len(events))
    assert len(events) >= 0
