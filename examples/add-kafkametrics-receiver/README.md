# Example of Chart Configuration

## Kafka Metrics Receiver
The [Kafka metrics receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kafkametricsreceiver)
collects Kafka metrics (brokers, topics, partitions, consumer groups) from kafka server, converting into otlp.

## How to deploy Zookeeper and Kafka with collector monitoring

### 1. Deploying Zookeeper and Kafka

Use the following command to deploy Kafka and Zookeeper to your Kubernetes cluster:

```bash
curl https://raw.githubusercontent.com/signalfx/splunk-otel-collector-chart/main/examples/add-kafkametrics-receiver/kafka.yaml | kubectl apply -f -
```

### 2. Configuring the Kafka Metrics Receiver

Update your values.yaml file with the following configuration to set up the kafkametricsreceiver:

```yaml
agent:
  config:
    receivers:
      kafkametrics:
        brokers: kafka-service.kafka.svc.cluster.local:9092
        protocol_version: 2.0.0
        scrapers:
          - brokers
          - topics
          - consumers
    service:
      pipelines:
        metrics:
          receivers: [ kafkametrics ]
```

### 3. Installing the Splunk OTel Collector Chart

With the configuration in place, deploy the Splunk OTel Collector using Helm:

```bash
helm install my-splunk-otel-collector --values my_values.yaml splunk-otel-collector-chart/splunk-otel-collector
```

### 4. Check out the results at [Splunk Observability](https://app.signalfx.com/#/metrics)

You can now view Kafka metrics in Splunk Observability.

## Checking the status and logs of the Kafka demo

Here are `kubectl` commands for checking the status and logs of the provided Kubernetes objects:

```bash
kubectl get deployment zookeeper-deployment -n kafka
kubectl logs -l app=zookeeper -n kafka
kubectl get deployment kafka-deployment -n kafka
kubectl logs -l app=kafka -n kafka
kubectl get service zookeeper-service -n kafka
kubectl get service kafka-service -n kafka
kubectl get cronjob kafka-producer-cronjob -n kafka
kubectl logs -l job-name=kafka-producer-cronjob -n kafka
kubectl get job kafka-consumer-job -n kafka
kubectl logs -l job-name=kafka-consumer-job -n kafka
```
