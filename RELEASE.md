## Release

### Versioning

Currently the helm chart version is mirroring major and minor version of the [Splunk OpenTelemetry
Collector](https://github.com/signalfx/splunk-otel-collector) image, e.g. if the chart uses 0.40.0 version of
Splunk OTel Collector image as default, the chart version should have 0.40.x version where x is a patch number.
This may be changed once Splunk OpenTelemetry Collector reaches GA.

Version of Splunk OTel Collector image is set as value of `appVersion` field in
[Chart.yaml](helm-charts/splunk-otel-collector/Chart.yaml), version of the helm chart release is set as value
of `version` field.

### Release Procedure

To make a new release of the helm chart:
1. Bump the `version` in [Chart.yaml](helm-charts/splunk-otel-collector/Chart.yaml) and create PR.
2. When the PR gets merged, the release will automatically be made and the helm repo updated.
3. Release notes are not populated automatically. So make sure to update them manually using the notes from
   [CHANGELOG](./CHANGELOG.md).
