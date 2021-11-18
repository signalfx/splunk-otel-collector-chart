"""
Copyright 2018-2019 Splunk, Inc..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
"""

import pytest
import time
import os
from .common import check_events_from_splunk

def pytest_addoption(parser):
    parser.addoption("--splunkd-url",
                     help="splunkd url used to send test data to. \
                          Eg: https://localhost:8089",
                     default="https://localhost:8089")
    parser.addoption("--splunk-user",
                     help="splunk username",
                     default="admin")
    parser.addoption("--splunk-password",
                     help="splunk user password",
                     default="password")

#
# def pytest_configure():
#     os.system('docker pull $CI_DATAGEN_IMAGE && kubectl apply -f test_setup.yaml')
#     time.sleep(60)

# Print events ingested in splunk for debugging
def pytest_unconfigure(config):
    indexes = ["main", "ci_events", "ns-anno", "pod-anno"]
    for index in indexes:
        search_query = "index=" + index + "  | fields *"
        events = check_events_from_splunk(start_time="-1h@h",
                                        url=config.getoption("--splunkd-url"),
                                        user=config.getoption("--splunk-user"),
                                        query=["search {0}".format(
                                            search_query)],
                                        password=config.getoption("--splunk-password"))
        print("index=" + index + " event count=" + str(len(events)))
        for event in events:
            print(event)

    metric_indexes = ["ci_metrics"]
    for index in metric_indexes:
        events = check_metrics_from_splunk(start_time="-24h@h",
                                      end_time="now",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      password=setup["splunk_password"],
                                      index=index_metrics,
                                      metric_name=metric)
        print("metric index=" + index + " event count=" + str(len(events)))
        for event in events:
            print(event)

@pytest.fixture(scope="function")
def setup(request):
    config = {}
    config["splunkd_url"] = request.config.getoption("--splunkd-url")
    config["splunk_user"] = request.config.getoption("--splunk-user")
    config["splunk_password"] = request.config.getoption("--splunk-password")


    return config
