# ObjectBucket Notifications Adapter

A Kubernetes operator that receives S3 bucket notifications from NooBaa (Multicloud Object Gateway) and/or Ceph RadosGW and dispatches them as CloudEvents to configured sink endpoints.

## Description

The ObjectBucket Notifications Adapter runs inside a Kubernetes cluster, registered as a `bucketNotifications` connection in NooBaa and/or as an SNS topic in RadosGW. It can receive notifications via HTTP (default) or by consuming from Kafka topics. It watches `ObjectBucketSource` custom resources to determine which S3 events from which buckets should be forwarded to which endpoints. When a notification arrives, the adapter matches the event against all configured sources and dispatches CloudEvents via HTTP or Kafka to the matching sinks.

The adapter determines whether an OBC is managed by NooBaa or RadosGW by matching the OBC's `spec.storageClassName` against configurable regex patterns, and uses the corresponding adapter ID and topic ARN when setting up bucket notifications.

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
| `NOOBAA_ADAPTER_ID` | `mcg-adapter` | Identifier used in the S3 bucket notification configuration for NooBaa-managed OBCs |
| `NOOBAA_ADAPTER_TOPIC_ARN` | `mcg-adapter-connection/connect.json` | NooBaa connection secret reference used as TopicArn in put-bucket-notification calls |
| `NOOBAA_ADAPTER_STORAGECLASS_PATTERN` | `.*noobaa\.io$` | Regex matched against OBC `spec.storageClassName` to classify as NooBaa-managed |
| `RADOSGW_ADAPTER_ID` | `rgw-adapter` | Identifier used in the S3 bucket notification configuration for RadosGW-managed OBCs |
| `RADOSGW_ADAPTER_TOPIC_ARN` | `arn:aws:sns:ocs-storagecluster-cephobjectstore::rgw-adapter-notifications` | RadosGW SNS TopicArn used in put-bucket-notification calls |
| `RADOSGW_ADAPTER_STORAGECLASS_PATTERN` | `.*ceph-rgw$` | Regex matched against OBC `spec.storageClassName` to classify as RadosGW-managed |
| `ADAPTER_PORT` | `8888` | Port the notification HTTP server listens on (HTTP mode only) |
| `NOTIFICATIONS_MODE` | `http` | `http` or `kafka` — selects how the adapter receives NooBaa/RadosGW notifications |
| `KAFKA_BROKERS` | _(none)_ | Comma-separated list of Kafka broker addresses. Required for Kafka sinks and when `NOTIFICATIONS_MODE=kafka`. |
| `KAFKA_SECRET` | _(none)_ | Name of a Kubernetes Secret (in the adapter's namespace) containing Kafka credentials. See `kafka-secret-format.md`. |
| `KAFKA_NOTIFICATIONS_TOPIC` | _(none)_ | Comma-separated list of Kafka topics to consume NooBaa/RadosGW notifications from (e.g. `mcg-adapter-notifications,rgw-adapter-notifications`). Required when `NOTIFICATIONS_MODE=kafka`. |
| `KAFKA_NOTIFICATIONS_GROUP_ID` | _(none)_ | Consumer group ID for consuming NooBaa/RadosGW notifications. Required when `NOTIFICATIONS_MODE=kafka`. |

### NooBaa Connection Setup (HTTP mode)

Before the adapter can receive notifications via HTTP, register it as an HTTP connection in NooBaa (one-time setup). A helper script `noobaa-connection-setup.sh` is provided. Manual steps:

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

As an alternative to HTTP, the adapter can consume NooBaa notifications from a Kafka topic. This requires a Strimzi-managed Kafka cluster. A helper script `noobaa-kafka-connection-setup.sh` is provided. See the `PLAN` file for detailed steps.

Summary of the one-time setup:

1. Create a `KafkaTopic` (e.g. `mcg-adapter-notifications`) and `KafkaUser` resources in Strimzi
2. Create a NooBaa connection secret (`mcg-adapter-connection`) with `notification_protocol: kafka` pointing to the Kafka bootstrap and topic
3. Patch the NooBaa CR to register the Kafka connection
4. Create a Kubernetes Secret in the adapter's namespace with the adapter's Kafka credentials (see `kafka-secret-format.md`)

### RadosGW Connection Setup (Kafka mode)

For Ceph RadosGW, bucket notifications are delivered via Kafka using SNS topics. A helper script `rook-kafka-connection-setup.sh` is provided. See the `PLAN` file for detailed steps.

Summary of the one-time setup:

1. Create a `KafkaTopic` (e.g. `rgw-adapter-notifications`) and `KafkaUser` resources in Strimzi
2. Create an SNS topic in RadosGW pointing to the Kafka broker and topic
3. Create a Kubernetes Secret in the adapter's namespace with the adapter's Kafka credentials (see `kafka-secret-format.md`)

### Deploy

```sh
make docker-build docker-push IMG=<some-registry>/objectbucket-notifications-adapter:tag
make install
make deploy IMG=<some-registry>/objectbucket-notifications-adapter:tag
```

To deploy in Kafka notifications mode (using default values from the PLAN):

```sh
make deploy-kafka IMG=<some-registry>/objectbucket-notifications-adapter:tag
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
