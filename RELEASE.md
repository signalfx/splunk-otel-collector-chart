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
1. Bump the `version` in [Chart.yaml](helm-charts/splunk-otel-collector/Chart.yaml)
2. Check for Helm Subchart version updates.
  - Look for a new version at https://github.com/open-telemetry/opentelemetry-operator/releases.
  - If needed, in the [Chart.yaml](helm-charts/splunk-otel-collector/Chart.yaml)
    update the operator version and run `helm dependency build`.
  - If the cert-manager subchart is updated, the helm-charts/splunk-otel-collector/crds/cert-manager.crds.yaml file
    will need to be updated as well to match. You can run `wget -P helm-charts/splunk-otel-collector/crds https://github.com/cert-manager/cert-manager/releases/download/{VERSION}/cert-manager.crds.yaml"`.
3. Run `make render` to render all the examples with the latest changes.
4. Create PR and request review from the team.
5. When the PR gets merged, the release will automatically be made and the helm repo updated.
6. Release notes are not populated automatically. So make sure to update them manually using the notes from
   [CHANGELOG](./CHANGELOG.md).
