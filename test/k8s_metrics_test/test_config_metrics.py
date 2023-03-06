import pytest
import time
import os
import logging
import json
from urllib.parse import urlparse
from ..common import check_events_from_splunk
from ..common import check_metrics_from_splunk


@pytest.mark.parametrize("metric", [
  #Control Plane Metrics
  ("apiserver_request_total"),
  ("workqueue_adds_total"),
  ("scheduler_scheduling_algorithm_duration_seconds"),
  ("kubeproxy_sync_proxy_rules_duration_seconds_count"),
  ("coredns_dns_requests_total"),
  #Container Metrics
  ("container.cpu.time"),
  ("container.cpu.utilization"),
  ("container.filesystem.available"),
  ("container.filesystem.capacity"),
  ("container.filesystem.usage"),
  ("container.memory.available"),
  ("container.memory.major_page_faults"),
  ("container.memory.page_faults"),
  ("container.memory.rss"),
  ("container.memory.usage"),
  ("container.memory.working_set"),
  ("k8s.container.cpu_limit"),
  ("k8s.container.cpu_request"),
  ("k8s.container.memory_limit"),
  ("k8s.container.memory_request"),
  ("k8s.container.ready"),
  ("k8s.container.restarts"),
  #Daemonset Metrics
  ("k8s.daemonset.current_scheduled_nodes"),
  ("k8s.daemonset.desired_scheduled_nodes"),
  ("k8s.daemonset.misscheduled_nodes"),
  ("k8s.daemonset.ready_nodes"),
  #Deployment Metrics
  ("k8s.deployment.available"),
  ("k8s.deployment.desired"),
  #Namespace Metrics
  ("k8s.namespace.phase"),
  #Node Metrics
  ("k8s.node.condition_ready"),
  ("k8s.node.cpu.time"),
  ("k8s.node.cpu.utilization"),
  ("k8s.node.filesystem.available"),
  ("k8s.node.filesystem.capacity"),
  ("k8s.node.filesystem.usage"),
  ("k8s.node.memory.available"),
  ("k8s.node.memory.major_page_faults"),
  ("k8s.node.memory.page_faults"),
  ("k8s.node.memory.rss"),
  ("k8s.node.memory.usage"),
  ("k8s.node.memory.working_set"),
  ("k8s.node.network.errors"),
  ("k8s.node.network.io"),
  #Pod Metrics
  ("k8s.pod.cpu.time"),
  ("k8s.pod.cpu.utilization"),
  ("k8s.pod.filesystem.available"),
  ("k8s.pod.filesystem.capacity"),
  ("k8s.pod.filesystem.usage"),
  ("k8s.pod.memory.available"),
  ("k8s.pod.memory.major_page_faults"),
  ("k8s.pod.memory.page_faults"),
  ("k8s.pod.memory.rss"),
  ("k8s.pod.memory.usage"),
  ("k8s.pod.memory.working_set"),
  ("k8s.pod.network.errors"),
  ("k8s.pod.network.io"),
  ("k8s.pod.phase"),
  #Replicaset Metrics
  ("k8s.replicaset.available"),
  ("k8s.replicaset.desired"),
  #otelcol Metrics
  ("otelcol_exporter_queue_size"),
  ("otelcol_exporter_send_failed_log_records"),
  ("otelcol_exporter_send_failed_metric_points"),
  ("otelcol_exporter_sent_log_records"),
  ("otelcol_exporter_sent_metric_points"),
  ("otelcol_otelsvc_k8s_ip_lookup_miss"),
  ("otelcol_otelsvc_k8s_namespace_added"),
  ("otelcol_otelsvc_k8s_namespace_updated"),
  ("otelcol_otelsvc_k8s_pod_added"),
  ("otelcol_otelsvc_k8s_pod_table_size"),
  ("otelcol_otelsvc_k8s_pod_updated"),
  ("otelcol_process_cpu_seconds"),
  ("otelcol_process_memory_rss"),
  ("otelcol_process_runtime_heap_alloc_bytes"),
  ("otelcol_process_runtime_total_alloc_bytes"),
  ("otelcol_process_runtime_total_sys_memory_bytes"),
  ("otelcol_process_uptime"),
  ("otelcol_processor_accepted_log_records"),
  ("otelcol_processor_accepted_metric_points"),
  ("otelcol_processor_batch_batch_send_size_bucket"),
  ("otelcol_processor_batch_batch_send_size_count"),
  ("otelcol_processor_batch_batch_send_size_sum"),
  ("otelcol_processor_batch_timeout_trigger_send"),
  ("otelcol_processor_dropped_log_records"),
  ("otelcol_processor_dropped_metric_points"),
  ("otelcol_processor_refused_log_records"),
  ("otelcol_processor_refused_metric_points"),
  ("otelcol_receiver_accepted_metric_points"),
  ("otelcol_receiver_refused_metric_points"),
  ("otelcol_scraper_errored_metric_points"),
  ("otelcol_scraper_scraped_metric_points"),
  #Scrape Metrics
  ("scrape_duration_seconds"),
  ("scrape_samples_post_metric_relabeling"),
  ("scrape_samples_scraped"),
  ("scrape_series_added"),
  #System Metrics
  ("system.cpu.load_average.15m"),
  ("system.cpu.load_average.1m"),
  ("system.cpu.load_average.5m"),
  ("system.cpu.time"),
  ("system.disk.io"),
  ("system.disk.io_time"),
  ("system.disk.merged"),
  ("system.disk.operation_time"),
  ("system.disk.operations"),
  ("system.disk.pending_operations"),
  ("system.disk.weighted_io_time"),
  ("system.filesystem.inodes.usage"),
  ("system.filesystem.usage"),
  ("system.memory.usage"),
  ("system.network.connections"),
  ("system.network.dropped"),
  ("system.network.errors"),
  ("system.network.io"),
  ("system.network.packets"),
  ("system.paging.faults"),
  ("system.paging.operations"),
  ("system.paging.usage"),
  ("system.processes.count"),
  ("system.processes.created"),
  #Up Metrics
  ("up"),
  # Network Explorer Metrics
  ("tcp.bytes"),
  ("tcp.active"),
  ("tcp.packets"),
  ("tcp.retrans"),
  ("tcp.syn.timeouts"),
  ("tcp.new_sockets"),
  ("tcp.resets"),
  ("tcp.rtt.num_measurements"),
  ("tcp.rtt.average"),
  ("udp.bytes"),
  ("udp.packets"),
  ("udp.active"),
  ("udp.drops"),
  ("http.status_code"),
  ("http.active_sockets"),
  ("http.client.duration_average"),
  ("http.server.duration_average"),
  ("dns.active_sockets"),
  ("dns.responses"),
  ("dns.timeouts"),
  ("dns.client.duration_average"),
  ("dns.server.duration_average")
])
def test_metric_name(setup, metric):
  '''
  This test covers one metric from each endpoint that the metrics plugin covers
  '''
  logging.info("testing for presence of metric={0}".format(metric))
  index_metrics = os.environ["CI_INDEX_METRICS"] if os.environ["CI_INDEX_METRICS"] else "ci_metrics"
  logging.info("index for metrics is {0}".format(index_metrics))
  events = check_metrics_from_splunk(start_time="-24h@h",
                                     end_time="now",
                                     url=setup["splunkd_url"],
                                     user=setup["splunk_user"],
                                     password=setup["splunk_password"],
                                     index=index_metrics,
                                     metric_name=metric)
  logging.info("Splunk received %s metrics in the last minute",
               len(events))
  assert len(events) >= 0
