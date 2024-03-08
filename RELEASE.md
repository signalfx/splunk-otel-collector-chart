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

#### Using GitHub Workflows

- **Manual Dispatch Github Worfklow:**
  - Navigate to the **[Release Drafter](https://github.com/signalfx/splunk-otel-collector-chart/actions/workflows/release_drafter.yaml** workflow under GitHub Actions.
  - Manually trigger the workflow. It automatically drafts a PR for the release.
  - Review code changes, validate chart functionality, approve the PR, and merge the PR.
- **Automatic Schedule Github Worfklow:**
  - Automatically generated PRs are scheduled to follow collector releases.
  - Review code changes, validate chart functionality, approve the PR, and merge the PR.

#### Manually Making a Release

1. **Version Update:** Manually edit [Chart.yaml](helm-charts/splunk-otel-collector/Chart.yaml) to update the `version` field.
1. **Dependencies & Rendering:** Execute `make render` to update Helm dependencies and apply changes.
1. **CHANGELOG Update:** Run `make chlog-update` to incorporate changes into the CHANGELOG.
1. **Stage & Commit Changes:**
   1. Stage all changes: `git add .`
   1. Commit with a message: `git commit -m "Prepare release {version}"`
1. **Create a pull request:**
   1. Push your commits to the signalfx owned remote repository.
   1. Create a PR for your changes against the main branch.
1. Review code changes, validate chart functionality, approve the PR, and merge the PR.
