## Release
To make a release bump the version in [Chart.yaml](helm-charts/splunk-otel-collector/Chart.yaml). When it gets merged the release will automatically be made and the helm repo updated.

If you have a dependent chart consuming this chart, bump the version dependency if needed and run `helm dep update`. Commit the changed and added files.
