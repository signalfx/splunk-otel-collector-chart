# Changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [0.24.10] - 2021-04-21

### Changed

- Disable fluentd metrics collection by default (#108)

## [0.24.9] - 2021-04-18

### Changed

- Change OTLP port from deprecated 55680 to default 4317 (#103)

### Fixed

- Open port for signalfx-forwarder on the agent (#106)

## [0.24.8] - 2021-04-16

### Fixed

- Fix traces enrichment with k8s metadata (#102)

## [0.24.7] - 2021-04-15

### Changed

- Switch to stable Splunk OTel Collector image 0.24.3 (#100)

## [0.24.6] - 2021-04-15

### Added

- Enable smartagent/signalfx-forwarder in the default agent trace pipeline (#98)

## [0.24.5] - 2021-04-13

### Added

- Enable batch processor in the default metrics pipelines (#90)

### Changed

- Ensure all metrics and traces are routed through the gateway deployment if
  it's enabled (#96)

## [0.24.4] - 2021-04-12

### Added

- Add an option to set extra environment variables (#91)

## [0.24.3] - 2021-04-12

### Fixed

- Fix resource attribute in the default traces pipeline (#88)
- Add metric_source:kubernetes for all k8s cluster metrics (#89)
- Fix host.name attribute in logs (#87)

## [0.24.2] - 2021-04-07

### Added

- Add host.name attribute to logs (#86)

## [0.24.1] - 2021-04-07

### Changed

- Remove deprecated opencensus receiver (#85)

## [0.24.0] - 2021-04-07

### Changed

- Upgrade image to 0.24.0 version (#84)
- Add system detector to default metrics and traces pipelines (#84)
