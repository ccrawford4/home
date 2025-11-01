# Redis Helm Chart

A simple and straightforward Helm chart for deploying a single instance Redis database on Kubernetes, based on the [Kubernetes guide for running single instance stateful applications](https://kubernetes.io/docs/tasks/run-application/run-single-instance-stateful-application/).

This chart can be used either as a standalone deployment or as a dependency in other Helm charts.

## Features

- Simple Redis single instance deployment
- Configurable instance name and namespace
- Persistent storage with PersistentVolumeClaim
- Secret management via external-secrets.io or static Kubernetes secrets
- Configurable resource limits
- Can be used as a dependency chart in other Helm applications

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- PersistentVolume provisioner support in the underlying infrastructure (for persistent storage)
- For external secrets: external-secrets.io installed and a ClusterSecretStore configured

## Installation

### As a Dependency Chart (Recommended)

This is the recommended way to use this chart. Include it as a dependency in your application's `Chart.yaml`:

```yaml
dependencies:
  - name: redis
    version: 0.1.0
    repository: "file://../../helm-library/redis"
```

Then configure it in your application's `values.yaml`:

```yaml
redis:
  name: my-app-redis
  namespace: my-app
  clusterSecretStoreName: cluster-secret-store
  image_name: redis  # or use image.repository
  service:
    type: ClusterIP
    port: 6379
  persistence:
    enabled: true
    size: 8Gi
  secrets:
    useExternalSecrets: true
    externalSecrets:
      passwordKey: my-app-redis-password
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 250m
      memory: 256Mi
```

After adding the dependency, run:

```bash
helm dependency update
helm install my-app ./my-app
```

### As a Standalone Chart

#### Using External Secrets (Recommended)

1. First, ensure you have your Redis password stored in your secret manager (e.g., Google Secret Manager, AWS Secrets Manager, etc.)

2. Create a values file (e.g., `my-redis-values.yaml`):

```yaml
name: my-redis-instance
namespace: my-namespace
secrets:
  useExternalSecrets: true
  externalSecrets:
    passwordKey: redis-password      # Key in your secret manager
```

3. Install the chart:

```bash
helm install my-redis ./helm-library/redis -f my-redis-values.yaml
```

#### Using Static Secrets

For testing or non-production environments, you can use static secrets:

1. Create a values file (e.g., `redis-static-values.yaml`):

```yaml
name: redis-test
namespace: redis-test
secrets:
  useExternalSecrets: false
  static:
    password: "changeMe123"
```

2. Install the chart:

```bash
helm install redis-test ./helm-library/redis -f redis-static-values.yaml
```

**WARNING:** Static secrets are stored in base64 in the cluster and are less secure. Use external secrets for production deployments.

## Configuration

The following table lists the configurable parameters of the Redis chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `name` | Redis instance name | `redis` |
| `namespace` | Kubernetes namespace | `redis` |
| `clusterSecretStoreName` | Name of the ClusterSecretStore for external secrets | `cluster-secret-store` |
| `image.repository` | Redis image repository | `redis` |
| `image.tag` | Redis image tag | `7.0` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image_name` | Alternative: Redis image name (overrides image.repository if set) | `""` |
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | Redis service port | `6379` |
| `persistence.enabled` | Enable persistent storage | `true` |
| `persistence.storageClass` | Storage class name | `""` (uses default) |
| `persistence.accessMode` | PVC access mode | `ReadWriteOnce` |
| `persistence.size` | PVC size | `8Gi` |
| `secrets.useExternalSecrets` | Use external-secrets.io | `true` |
| `secrets.externalSecrets.passwordKey` | Secret manager key for Redis password | `redis-password` |
| `secrets.static.password` | Static password (when useExternalSecrets is false) | `""` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.requests.cpu` | CPU request | `250m` |
| `resources.requests.memory` | Memory request | `256Mi` |

## Creating Multiple Redis Instances

You can easily deploy multiple Redis instances by using different values files:

```bash
# Instance 1
helm install redis-app1 ./helm-library/redis -f redis-app1-values.yaml

# Instance 2
helm install redis-app2 ./helm-library/redis -f redis-app2-values.yaml
```

Just make sure to use different `name` and `namespace` values in each values file.

## Connecting to Redis

Once deployed, you can connect to Redis from within the cluster using:

```
Host: <name>.<namespace>.svc.cluster.local
Port: 6379
Password: <configured password>
```

For example, with the default values:
```
Host: redis.redis.svc.cluster.local
Port: 6379
```

## Uninstalling

To uninstall/delete the deployment:

```bash
helm uninstall my-redis
```

**Note:** This will not delete the PersistentVolumeClaim. To delete the PVC and associated data:

```bash
kubectl delete pvc <name>-pvc -n <namespace>
# Example: kubectl delete pvc my-redis-instance-pvc -n my-namespace
```

## Example: Integration with Applications

See the `search-app` example in this repository for a complete example of how to use this chart as a dependency:

```yaml
# Chart.yaml
dependencies:
  - name: redis
    version: 0.1.0
    repository: "file://../../helm-library/redis"

# values.yaml
redis:
  name: search-app-redis
  namespace: search-app
  secrets:
    useExternalSecrets: true
    externalSecrets:
      passwordKey: search-app-redis-password

# In your application deployment:
deployments:
  - name: my-app
    image:
      repository: myapp/backend
      tag: "latest"
    container:
      env:
        - name: REDIS_HOST
          value: "search-app-redis.search-app.svc.cluster.local"
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: search-app-redis-secrets
              key: REDIS_PASSWORD
```
