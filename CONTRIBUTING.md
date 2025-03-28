# Contributing Guidelines

Thank you for your interest in contributing to our project! Whether it's a bug
report, new feature, question, or additional documentation, we greatly value
feedback and contributions from our community. Read through this document
before submitting any issues or pull requests to ensure we have all the
necessary information to effectively respond to your bug report or
contribution.

In addition to this document, please review our [Code of
Conduct](CODE_OF_CONDUCT.md). For any code of conduct questions or comments
please email oss@splunk.com.

## Reporting Bugs/Feature Requests

We welcome you to use the GitHub issue tracker to report bugs or suggest
features. When filing an issue, please check existing open, or recently closed,
issues to make sure somebody else hasn't already reported the issue. Please try
to include as much information as you can. Details like these are incredibly
useful:

- A reproducible test case or series of steps
- The version of our code being used
- Any modifications you've made relevant to the bug
- Anything unusual about your environment or deployment
- Any known workarounds

When filing an issue, please do *NOT* include:

- Internal identifiers such as JIRA tickets
- Any sensitive information related to your environment, users, etc.

## Documentation

The Splunk Observability documentation is hosted on https://docs.splunk.com/Observability,
which contains all the prescriptive guidance for Splunk Observability products.
Prescriptive guidance consists of step-by-step instructions, conceptual material,
and decision support for customers. Reference documentation and development
documentation is hosted on this repository.

You can send feedback about Splunk Observability docs by clicking the Feedback
button on any of our documentation pages.

## Contributing via Pull Requests

We appreciate your contributions! Here’s a breakdown of the steps based on your relationship with the owner SignalFX organization:

### Case 1: Public Contributors (No Connection to SignalFX Organization)

As a **public contributor** with no relation to SignalFX organization, you will likely **not have permission to create a feature branch** in the SignalFX repository. Here’s what to do:

1. **Fork the repository**: Create a fork of the repository and work on your own feature branch.
2. **Submit a PR from your fork**: Open a PR from your forked repository.
3. **Request help from SignalFX Team**: Since you don't have the required permissions to create a github.com/signalfx/splunk-otel-collector-chart owned feature branch, kindly request help from the SignalFX team members who are code owners. We will assist in getting your PR set up for proper testing.
4. **Security and testing**: We may need to slightly modify or re-create your PR to ensure it works with our live Kubernetes test clusters, as this is a security requirement for accessing these clusters.

### Case 2: SignalFX Members (New to the Project)

As a **SignalFX team member** who is new to this project, you are allowed to work with forks for personal work, but for merging changes into the main repository, you must follow these steps:

1. **Fork for personal work**: Continue using your fork for personal development and initial testing.
2. **Create a PR with SignalFX-owned feature branch**: When you're ready to submit a PR to the main repository, create a feature branch **owned by github.com/signalfx/splunk-otel-collector-chart**. This ensures that the **functional tests** can run on our live test clusters (EKS, GKE, AKS).
3. **Get help if needed**: If you're unsure about how to create the feature branch under the SignalFX organization, just ask the team for assistance.

### Case 3: Regular Contributors (Co-Owners of the Project)

As a **regular contributor** who is a co-owner of the repository, you are fully responsible for maintaining and supporting the project. You will follow these steps:

1. **Create SignalFX-owned feature branches**: Continue managing and maintaining the repository using **github.com/signalfx/splunk-otel-collector-chart owned feature branches**.
2. **Run functional tests**: Ensure that all your PRs pass full validation, including tests against real Kubernetes test clusters (EKS, GKE, AKS), to ensure everything is working as expected.
3. **Support the community**: Take responsibility for the community by maintaining test coverage, supporting contributors, and helping new contributors integrate with the project.

### Best Practices for PRs

Contributions via Pull Requests (PRs) are much appreciated. Before sending us a pull request, please ensure that:

1. **Work against the latest `main` branch**: Ensure your branch is up to date with the latest code in the `main` branch.
2. **Check for duplicate work**: Review existing open and recently merged PRs to avoid duplicating someone else’s work.
4. **Keep your PR manageable**: Ideally, submit PRs that are **less than 500 lines of code**. For larger contributions, consider splitting them into multiple PRs to make review easier.

To send us a pull request, please:

1. **Fork the repository**: Begin by forking the repository.
2. **Make a single change per PR**: Focus on a **single change** per PR. If you need to reformat code, do it in a separate PR.
3. **Follow versioning instructions**: Refer to the [RELEASE.md](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/RELEASE.md) for versioning guidelines.
4. **Ensure tests pass**: Run tests locally and add new tests as needed for your contribution.
5. **Update the CHANGELOG**: If your PR alters functionality, add an entry to `CHANGELOG.md` using `make chlog-new`. Refer to the [Changelog Guidelines](https://signalfx/splunk-otel-collector-chart/blob/main/CONTRIBUTING.md#changelog-guidelines).
6. **Render example documentation**: If chart templates are updated, run `make render` to generate updated documentation.
7. **Commit with clear messages**: Use **clear commit messages** that describe your changes.
8. **Submit the PR**: Open a PR and answer any default questions in the PR interface.
9. **Engage with CI feedback**: Pay attention to any automated CI failures reported in your PR and stay involved in the conversation to resolve issues.

GitHub provides additional documentation on [forking a
repository](https://help.github.com/articles/fork-a-repo/) and [creating a pull
request](https://help.github.com/articles/creating-a-pull-request/).

## Changelog Guidelines

### When to Add an Entry

Include a changelog entry for pull requests affecting:

1. Collector configuration or behavior
2. Telemetry data output

**Exceptions:**

- Documentation-only changes
- Minor, non-impactful updates (e.g., code cleanup)

### Adding an Entry

**Quick Guide:**

1. **Create File:** Run `make chlog-new` to generate a `.yaml` in `./.chloggen`.
2. **Edit File:** Update the `.yaml` with relevant info.
3. **Validate:** Use `make chlog-validate` to check format.
4. **Commit:** Add the `.yaml` to your pull request.

**Manual Option:**

- Copy `./.chloggen/TEMPLATE.yaml` or create a unique `.yaml` file.

## Finding contributions to work on

Looking at the existing issues is a great way to find something to contribute
on. As our projects, by default, use the default GitHub issue labels
(enhancement/bug/duplicate/help wanted/invalid/question/wontfix), looking at
any 'help wanted' issues is a great place to start.

## Building

When changing the helm chart the files under `examples/*/rendered_manifests` need to be rebuilt with `make render`. It's strongly recommended to use [pre-commit](https://pre-commit.com/) which will do this automatically for each commit (as well as run some linting).

## Running locally

It's recommended to use [minikube](https://github.com/kubernetes/minikube) with
[calico networking](https://docs.projectcalico.org/getting-started/kubernetes/) to run splunk-otel-collector locally.

If you run it on Windows or MacOS, use a linux VM driver, e.g. virtualbox.
In that case use the following arguments to start minikube cluster:

```bash
minikube start --cni calico --vm-driver=virtualbox
```

### Troubleshooting

In some local Kubernetes clusters like "minikube" and "kind", you might run into TLS verification issue when callig
the kubelet API. In order to quickly resolve it add the following section to your values.yaml file:

```yaml
agent:
  config:
    receivers:
      kubeletstats:
        insecure_skip_verify: true
```

## Licensing

See the [LICENSE](LICENSE) file for our project's licensing. We will ask you to
confirm the licensing of your contribution.

We may ask you to sign a [Contributor License Agreement
(CLA)](http://en.wikipedia.org/wiki/Contributor_License_Agreement) for larger
changes.
