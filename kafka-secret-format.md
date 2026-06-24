# Kafka Auth Secret Format

Reference for the Kubernetes Secret format to configure Kafka client connections.

Originally defined in eventing-kafka-broker `control-plane/pkg/security/secret.go`

## Required Key

| Key | Values | Description |
|---|---|---|
| `protocol` | `PLAINTEXT`, `SASL_PLAINTEXT`, `SSL`, `SASL_SSL` | Selects the security protocol. Required in every secret. |

The protocol determines which additional keys are expected:

| Protocol | SASL keys | TLS keys |
|---|---|---|
| `PLAINTEXT` | -- | -- |
| `SASL_PLAINTEXT` | yes | -- |
| `SSL` | -- | yes |
| `SASL_SSL` | yes | yes |

## SASL Keys

Used when protocol is `SASL_PLAINTEXT` or `SASL_SSL`.

| Key | Required | Values / Format | Default | Description |
|---|---|---|---|---|
| `sasl.mechanism` | no | `PLAIN`, `SCRAM-SHA-256`, `SCRAM-SHA-512`, `OAUTHBEARER` | `PLAIN` | SASL mechanism |
| `user` | yes | string | -- | SASL username |
| `password` | yes | string | -- | SASL password |

## TLS Keys

Used when protocol is `SSL` or `SASL_SSL`.

| Key | Required | Format | Description |
|---|---|---|---|
| `ca.crt` | no | PEM-encoded certificate | CA certificate for server verification. If omitted, the system root CA set is used. |
| `user.crt` | conditional | PEM-encoded certificate | Client certificate for mTLS. Required when protocol is `SSL` and `user.skip` is not `true`. Not used for `SASL_SSL`. |
| `user.key` | conditional | PEM-encoded private key (PKCS#8) | Client private key for mTLS. Same conditions as `user.crt`. PKCS#1 format is rejected. |
| `user.skip` | no | `true` or `false` | When `true`, skips the client certificate requirement for protocol `SSL`. Default `false`. |

Note: `user.crt` and `user.key` (mTLS) are only required/checked when `protocol` is `SSL`. When `protocol` is `SASL_SSL`, only `ca.crt` is relevant for TLS.

## Examples

### PLAINTEXT (no auth)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kafka-credentials
type: Opaque
stringData:
  protocol: PLAINTEXT
```

### SASL_PLAINTEXT with SCRAM-SHA-512

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kafka-credentials
type: Opaque
stringData:
  protocol: SASL_PLAINTEXT
  sasl.mechanism: SCRAM-SHA-512
  user: my-user
  password: my-password
```

### SSL with mTLS

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kafka-credentials
type: Opaque
stringData:
  protocol: SSL
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
  user.crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
  user.key: |
    -----BEGIN PRIVATE KEY-----
    ...
    -----END PRIVATE KEY-----
```

### SSL with server-only TLS (no client cert)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kafka-credentials
type: Opaque
stringData:
  protocol: SSL
  user.skip: "true"
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

### SASL_SSL with SCRAM-SHA-512

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kafka-credentials
type: Opaque
stringData:
  protocol: SASL_SSL
  sasl.mechanism: SCRAM-SHA-512
  user: my-user
  password: my-password
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

