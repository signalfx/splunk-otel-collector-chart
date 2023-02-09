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

Contributions via Pull Requests (PRs) are much appreciated. Before sending us a
pull request, please ensure that:

1. You are working against the latest source on the `main` branch.
2. You check existing open, and recently merged, pull requests to make sure
   someone else hasn't addressed the problem already.
3. You open an issue to discuss any significant work - we would hate for your
   time to be wasted.
4. You submit PRs that are easy to review and ideally less 500 lines of code.
   Multiple PRs can be submitted for larger contributions.

To send us a pull request, please:

1. Fork the repository.
2. Modify the source; please ensure a single change per PR. If you also
   reformat all the code, it will be hard for us to focus on your change.
3. Please follow the versioning instructions found in the [RELEASE.md](https://github.com/signalfx/splunk-otel-collector-chart/blob/main/RELEASE.md).
4. Ensure local tests pass and add new tests related to the contribution.
5. Commit to your fork using clear commit messages.
6. Send us a pull request, answering any default questions in the pull request
   interface.
7. Pay attention to any automated CI failures reported in the pull request, and
   stay involved in the conversation.

GitHub provides additional documentation on [forking a
repository](https://help.github.com/articles/fork-a-repo/) and [creating a pull
request](https://help.github.com/articles/creating-a-pull-request/).

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
> :warn: Adding or removing components is not officially supported by Splunk as
> it may change performance characteristics and/or system behavior. Support is
> provided if issues experienced can be reproduced with official builds.

## Licensing

See the [LICENSE](LICENSE) file for our project's licensing. We will ask you to
confirm the licensing of your contribution.

We may ask you to sign a [Contributor License Agreement
(CLA)](http://en.wikipedia.org/wiki/Contributor_License_Agreement) for larger
changes.
