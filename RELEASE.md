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
1. Bump the chart `version` in [Chart.yaml](helm-charts/splunk-otel-collector/Chart.yaml)
1. Bump dependencies versions as needed
   - Look for new releases
     - https://cert-manager.io/docs/installation/supported-releases/
     - https://github.com/open-telemetry/opentelemetry-operator/releases
   - Increment versions under `dependencies` in [Chart.yaml](helm-charts/splunk-otel-collector/Chart.yaml#)
1. Run `make render demo-update` to update Helm dependencies, render all the examples, and update the demos with the latest changes.
1. Create PR and request review from the team.
1. When the PR gets merged, the release will automatically be made and the helm repo updated.
1. Release notes are not populated automatically. So make sure to update them manually using the notes from [CHANGELOG](./CHANGELOG.md).
