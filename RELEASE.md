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

1. When a new upstream collector release is available, the [Release Drafter workflow](https://github.com/signalfx/splunk-otel-collector-chart/actions/workflows/release_drafter.yaml) automatically creates a release PR every 12 hours at **00:55 UTC and 12:55 UTC**.
   - This schedule corresponds to:
     - **5:55 AM / 5:55 PM** during Daylight Saving Time (PDT / UTC−7)
     - **4:55 AM / 4:55 PM** during Standard Time (PST / UTC−8)
   - To trigger the workflow outside the scheduled times, use the ["Run workflow" button](https://github.com/signalfx/splunk-otel-collector-chart/actions/workflows/release_drafter.yaml) with default input parameters.
1. After the [release PR](https://github.com/signalfx/splunk-otel-collector-chart/pulls?q=is%3Apr+is%3Aopen+%22Prepare+Release%22) is created:
   - Review the code changes
   - Ensure all checks have passed to validate chart functionality
     - Note: Due to a GitHub limitation, PRs created by workflows may not trigger downstream checks automatically. A release approver may need
       to close and reopen the PR to kick off the required checks. This is a known issue we’re working to improve.
   - Approve and merge the PR
1. The [Release Charts workflow](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/.github/workflows/release.yaml) will publish the release shortly after.
   - That’s it — you’re done!
   - _Automation prevents duplicate or invalid releases. If a release isn’t created, you can check for failed workflow run info [here](https://github.com/signalfx/splunk-otel-collector-chart/actions/workflows/release.yaml)._

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
