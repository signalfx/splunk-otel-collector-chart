# Example of chart configuration

## Route log records to a specific Splunk Enterprise or Splunk Cloud Platform index

When exporting logs to a Splunk Enterprise or Splunk Cloud Platform endpoint, a
user can set configurations so logs that are ingested into a specific index. In
this example we configure log collection settings so that logs generated from a
given Kubernetes namespace will be collected, exported to the endpoint, and
indexed into an index with the same name as the Kubernetes namespace.
