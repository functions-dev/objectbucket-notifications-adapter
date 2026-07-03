# MCG Adapter

A Kubernetes operator that receives S3 bucket notifications from NooBaa (Multicloud Object Gateway) and dispatches them as CloudEvents to configured sink endpoints.

## Description

The MCG Adapter runs inside a Kubernetes cluster, registered as a `bucketNotifications` connection in NooBaa. It can receive notifications via HTTP (default) or by consuming from a Kafka topic. It watches `ObjectBucketSource` custom resources to determine which S3 events from which buckets should be forwarded to which endpoints. When NooBaa delivers a notification, the adapter matches the event against all configured sources and dispatches CloudEvents via HTTP or Kafka to the matching sinks.

## Custom Resource: ObjectBucketSource

```yaml
apiVersion: sources.functions.dev/v1alpha1
kind: ObjectBucketSource
metadata:
  namespace: foobar
  name: foo
spec:
  objectBucketClaim:
    name: foo-bucket          # ObjectBucketClaim in the same namespace
  events:
  - "s3:ObjectCreated:*"      # S3 event types to subscribe to
  sink:
    uri: http://foo.foobar.svc.cluster.local
```

The sink URI can be an HTTP endpoint or a Kafka topic reference (e.g. `kafka:foo-topic`). The Kafka broker(s) to connect to are configured globally via the `KAFKA_BROKERS` environment variable.

The controller manages three status conditions:

| Condition | Meaning |
|---|---|
| `OBCCredentialsAvailable` | The OBC's ConfigMap and Secret exist with the expected keys |
| `BucketNotificationSet` | The S3 bucket notification was configured in NooBaa |
| `TestEventReceived` | The test event sent by NooBaa after notification setup was received |

Multiple `ObjectBucketSource` resources referencing the same OBC are merged into a single bucket notification configuration (union of event types). Each source only receives events matching its own subscription.

## Getting Started

### Prerequisites

- Go 1.24+
- Docker 17.03+
- kubectl v1.11.3+
- Access to a Kubernetes cluster with NooBaa installed

### Configuration

The adapter is configured via environment variables:

| Variable | Default | Description |
|---|---|---|
| `ADAPTER_ID` | `mcg-adapter` | Identifier used in the S3 bucket notification configuration |
| `ADAPTER_TOPIC` | `mcg-adapter-connection/connect.json` | NooBaa connection secret reference used as TopicArn in put-bucket-notification calls |
| `ADAPTER_PORT` | `8888` | Port the notification HTTP server listens on (HTTP mode only) |
| `NOTIFICATIONS_MODE` | `http` | `http` or `kafka` — selects how the adapter receives NooBaa notifications |
| `KAFKA_BROKERS` | _(none)_ | Comma-separated list of Kafka broker addresses. Required for Kafka sinks and when `NOTIFICATIONS_MODE=kafka`. |
| `KAFKA_SECRET` | _(none)_ | Name of a Kubernetes Secret (in the adapter's namespace) containing Kafka credentials. See `kafka-secret-format.md`. |
| `KAFKA_NOTIFICATIONS_TOPIC` | _(none)_ | Kafka topic to consume NooBaa notifications from. Required when `NOTIFICATIONS_MODE=kafka`. |
| `KAFKA_NOTIFICATIONS_GROUP_ID` | _(none)_ | Consumer group ID for consuming NooBaa notifications. Required when `NOTIFICATIONS_MODE=kafka`. |

### NooBaa Connection Setup (HTTP mode)

Before the adapter can receive notifications via HTTP, register it as an HTTP connection in NooBaa (one-time setup):

1. Create the connection secret:

```sh
oc create secret generic mcg-adapter-connection \
  --from-file=connect.json=/dev/stdin -n openshift-storage <<EOF
{
  "name": "mcg-adapter-connection",
  "notification_protocol": "http",
  "agent_request_object": {
    "host": "<adapter-service>.<adapter-namespace>.svc.cluster.local",
    "port": 8888
  }
}
EOF
```

2. Patch the NooBaa CR to register the connection:

```sh
existing_connections=$(oc get noobaa noobaa -n openshift-storage -o json \
  | jq -c '.spec.bucketNotifications.connections // []')

updated_connections=$(echo "$existing_connections" | jq -c \
  --arg name "mcg-adapter-connection" \
  '[.[] | select(.name != $name)] + [{"name": $name, "namespace": "openshift-storage"}]')

oc patch noobaa noobaa --type='merge' -n openshift-storage -p '{
  "spec": {
    "bucketNotifications": {
      "connections": '"${updated_connections}"',
      "enabled": true
    }
  }
}'
```

### NooBaa Connection Setup (Kafka mode)

As an alternative to HTTP, the adapter can consume NooBaa notifications from a Kafka topic. This requires a Strimzi-managed Kafka cluster. See the `PLAN` file for detailed setup steps, including creating the KafkaTopic, KafkaUsers, and the NooBaa Kafka connection secret.

Summary of the one-time setup:

1. Create a `KafkaTopic` (e.g. `mcg-adapter-notifications`) and `KafkaUser` resources in Strimzi
2. Create a NooBaa connection secret with `notification_protocol: kafka` pointing to the Kafka bootstrap and topic
3. Patch the NooBaa CR to register the Kafka connection
4. Create a Kubernetes Secret in the adapter's namespace with the adapter's Kafka credentials (see `kafka-secret-format.md`)

### Deploy

```sh
make docker-build docker-push IMG=<some-registry>/mcg-adapter:tag
make install
make deploy IMG=<some-registry>/mcg-adapter:tag
```

To deploy in Kafka notifications mode (using default values from the PLAN):

```sh
make deploy-kafka IMG=<some-registry>/mcg-adapter:tag
```

The `deploy-kafka` target uses the kustomize overlay in `config/kafka/` which sets `NOTIFICATIONS_MODE=kafka` along with default Kafka broker, topic, and secret values. Edit `config/kafka/kafka_env_patch.yaml` to customize these for your cluster.

### Run Locally (development)

```sh
make install          # Install CRDs
make run              # Run the operator locally
```

### Uninstall

```sh
kubectl delete -k config/samples/
make uninstall
make undeploy
```

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
