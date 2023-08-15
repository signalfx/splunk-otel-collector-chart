# Examples of Kafkametricsreceiver configurations in Splunk Observability


## 1. Modify Yaml file

For configuring Kafkametricsreceiver, please refer to value.yaml and add following configuration to your path-to-values-file.yaml.

You can add your what you want to measure in scrapers.

```
config:
    receivers:
      kafkametrics:
        brokers: kafka-service:9092
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

## 2. Set up Zookeepr and Kafka

Use Kubernetes to set up Kafka, Zookeeper.

```
# Create namespace
kubectl apply -f kafka-namespace.yaml

# Change to current namespace
kubectl config set-context --current --namespace=kafka

kubectl apply -f zookeeper.yaml

kubectl apply -f kafka-deployment.yaml

kubectl apply -f kafka-service.yaml
```

## 3. Create Kafka topics

Use ```kubectl get pod``` to obtain the pod ID, and use this ID with Kafka to create topics.

Example:
```
pod/kafka-deployment-85c5959f8b-h948r   1/1     Running   0          58s

kubectl exec -it kafka-deployment-85c5959f8b-h948r  -- /bin/bash

kafka-topics --create --bootstrap-server localhost:29092 --replication-factor 1 --partitions 1 --topic topic-name

kafka-console-consumer --bootstrap-server localhost:29092 --topic topic-name
```
## 4. Set up Python

Set up Python in Kubernetes.

```
kubectl run python-repl --rm -i -t --image=python:3.9 -- /bin/bash

# install libary
pip install confluent_kafka
```

```
# let Python connect to Kafka
from confluent_kafka import Producer
import socket
conf = {"bootstrap.servers": "kafka-service:9092", "client.id": socket.gethostname()}
producer = Producer(conf)
producer.produce("topic-name", key="message", value="message_from_python_producer")
```

## 5. Set up splunk-otel-collector-chart

Now, everything set up. Use helm to install splunk-otel-collector-chart.

```
helm install my-splunk-otel-collector --values path-to-values-file.yaml splunk-otel-collector-chart/splunk-otel-collector
```

You can check now Kafka metrics on Splunk Observability through Metric Finder.


# Additional Notes

The following instructions include extra settings for confirming the exported data on Splunk Observability, if you make changes to your own opentelemetry-collector-contrib and want to use splunk-otel-collector.

## 1. Replace Go module

In the file splunk-otel-collector/go.mod, replace the moduleFormat with your module format.

```
# Format : replace opentelemetry-collector-contrib/receivers version => myrepo/receivers version
ex: replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver version => github.com/xxxx/opentelemetry-collector-contrib/receiver/kafkametricsreceiver version
```

Use ``` go mod tidy``` for downloading dependency modules.

## 2. Push Docker image

Push splunk-otel-collector to your Docker image.

```
make docker-otelcol

docker tag otelcol:amd64 docker-user-name/image-name:latest

docker push docker-user-name/image-name:latest
```


## 3. Modify Yaml file
Please refer to value.yaml and add following configuration to your path-to-values-file.yaml for using your Docker image.

```
otelcol:

    # The registry and name of the opentelemetry collector image to pull
    repository: docker-user-name/image-name

    # The tag of the Splunk OTel Collector image, default value is the chart appVersion
    tag: "latest"

    # The policy that specifies when the user wants the opentelemetry collector images to be pulled

    pullPolicy: Always
```

## 4. Set up splunk-otel-collector-chart
Now, everything set up. Use helm to install splunk-otel-collector-chart.

```
helm install splunk-otel-collector --values your-values.yaml ./helm-charts/splunk-otel-collector
```

# Reference
* https://github.com/signalfx/splunk-otel-collector-chart

* https://opentelemetry.io/
