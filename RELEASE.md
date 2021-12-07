## Release

To make a new release of the helm chart:
1. Bump the version in [Chart.yaml](helm-charts/splunk-otel-collector/Chart.yaml) and create PR. 
2. When the PR gets merged, the release will automatically be made and the helm repo updated.
3. Release notes are not populated automatically. So make sure to update them manually using the notes from [CHANGELOG](./CHANGELOG.md).
