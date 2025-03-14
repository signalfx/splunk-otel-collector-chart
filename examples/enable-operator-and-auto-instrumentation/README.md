# OpenTelemetry Operator and Auto-Instrumentation Example Guide

In this guide, we provide several demos and examples that demonstrate different approaches to
instrumenting projects using this chart and the OpenTelemetry Operator for various applications.

## [Spring PetClinic - Java Instrumentation](./spring-petclinic-java.md)
This example demonstrates how to:
- Deploy the chart and demo into the current namespace.
- Instrument multiple applications which interact with each other.

**Highlights:**
- **Deep Insights:** Explore application relations and traces in the APM console with a rich, interconnected application dataset.
- **Multi-app Instrumentation:** Get a comprehensive view of transactions across multiple applications.

## [OpenTelemetry Demo - NodeJS Instrumentation](./otel-demo-nodejs.md)
This example demonstrates how to:
- Deploy the chart to the current namespace and the demo to the `otel-demo` namespace.
- Instrument a single NodeJS application.

**Highlights:**
- **Single App Focus:** Explore trace-related performance of a single instrumented NodeJS application in the APM console.
- **Simplified Use Case:** Although relations between applications will not be showcased in the APM console, this demo offers a simplified setup suitable for understanding basic instrumentation and trace visualization.

## [Simple Webserver - .NET Instrumentation](./otel-demo-nodejs.md)
This example demonstrates how to:
- Deploy the chart to the current namespace and the demo to the `dotnet-demo` namespace.
- Instrument a single .NET application.

**Highlights:**
- **Single App Focus:** Explore trace-related performance of a single instrumented .NET application in the APM console.
- **Simplified Use Case:** Although relations between applications will not be showcased in the APM console, this demo offers a simplified setup suitable for understanding basic instrumentation and trace visualization.

## Exploring Traces and Applications in APM Console
The examples provide practical insights into using the APM console for exploring application relations and traces.
Whether dealing with multiple applications interacting with each other or focusing on a single application, you will gain hands-on experience in visualizing trace data using Splunk Observability APM.

## Additional Resources
- [Getting Started with Auto-Instrumentation](../../docs/auto-instrumentation-install.md)
