# MySQL Helm Chart

A simple and straightforward Helm chart for deploying a single instance MySQL database on Kubernetes, based on the [Kubernetes guide for running single instance stateful applications](https://kubernetes.io/docs/tasks/run-application/run-single-instance-stateful-application/).

## Features

- Simple MySQL single instance deployment
- Configurable instance name and namespace
- Persistent storage with PersistentVolumeClaim
- Secret management via external-secrets.io or static Kubernetes secrets
- Configurable resource limits
- Optional database creation on first run

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- PersistentVolume provisioner support in the underlying infrastructure (for persistent storage)
- For external secrets: external-secrets.io installed and a ClusterSecretStore configured

## Installation

### Using External Secrets (Recommended)

1. First, ensure you have your MySQL credentials stored in your secret manager (e.g., Google Secret Manager, AWS Secrets Manager, etc.)

2. Create a values file (e.g., `my-mysql-values.yaml`):

```yaml
name: my-mysql-instance
namespace: my-namespace
mysql:
  database: mydatabase  # Optional: database to create on first run
secrets:
  useExternalSecrets: true
  externalSecrets:
    rootPasswordKey: mysql-root-password      # Key in your secret manager
    userPasswordKey: mysql-user-password      # Key in your secret manager
    usernameKey: mysql-username               # Key in your secret manager
```

3. Install the chart:

```bash
helm install my-mysql ./helm/mysql -f my-mysql-values.yaml
```

### Using Static Secrets

For testing or non-production environments, you can use static secrets:

1. Create a values file (e.g., `mysql-static-values.yaml`):

```yaml
name: mysql-test
namespace: mysql-test
mysql:
  database: testdb
secrets:
  useExternalSecrets: false
  static:
    rootPassword: "changeMe123"
    username: "testuser"
    password: "testPass456"
```

2. Install the chart:

```bash
helm install mysql-test ./helm/mysql -f mysql-static-values.yaml
```

**WARNING:** Static secrets are stored in base64 in the cluster and are less secure. Use external secrets for production deployments.

## Configuration

The following table lists the configurable parameters of the MySQL chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `name` | MySQL instance name | `mysql` |
| `namespace` | Kubernetes namespace | `mysql` |
| `clusterSecretStoreName` | Name of the ClusterSecretStore for external secrets | `cluster-secret-store` |
| `image.repository` | MySQL image repository | `mysql` |
| `image.tag` | MySQL image tag | `8.0` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | MySQL service port | `3306` |
| `persistence.enabled` | Enable persistent storage | `true` |
| `persistence.storageClass` | Storage class name | `""` (uses default) |
| `persistence.accessMode` | PVC access mode | `ReadWriteOnce` |
| `persistence.size` | PVC size | `20Gi` |
| `mysql.database` | Optional database to create on first run | `""` |
| `secrets.useExternalSecrets` | Use external-secrets.io | `true` |
| `secrets.externalSecrets.rootPasswordKey` | Secret manager key for root password | `mysql-root-password` |
| `secrets.externalSecrets.userPasswordKey` | Secret manager key for user password | `mysql-user-password` |
| `secrets.externalSecrets.usernameKey` | Secret manager key for username | `mysql-username` |
| `secrets.static.rootPassword` | Static root password (when useExternalSecrets is false) | `""` |
| `secrets.static.username` | Static username (when useExternalSecrets is false) | `""` |
| `secrets.static.password` | Static user password (when useExternalSecrets is false) | `""` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `1Gi` |
| `resources.requests.cpu` | CPU request | `250m` |
| `resources.requests.memory` | Memory request | `512Mi` |

## Creating Multiple MySQL Instances

You can easily deploy multiple MySQL instances by using different values files:

```bash
# Instance 1
helm install mysql-app1 ./helm/mysql -f mysql-app1-values.yaml

# Instance 2
helm install mysql-app2 ./helm/mysql -f mysql-app2-values.yaml
```

Just make sure to use different `name` and `namespace` values in each values file.

## Connecting to MySQL

Once deployed, you can connect to MySQL from within the cluster using:

```
Host: <name>.<namespace>.svc.cluster.local
Port: 3306
Username: <configured username>
Password: <configured password>
```

For example, with the default values:
```
Host: mysql.mysql.svc.cluster.local
Port: 3306
```

## Uninstalling

To uninstall/delete the deployment:

```bash
helm uninstall my-mysql
```

**Note:** This will not delete the PersistentVolumeClaim. To delete the PVC and associated data:

```bash
kubectl delete pvc <name>-pvc -n <namespace>
# Example: kubectl delete pvc my-mysql-instance-pvc -n my-namespace
```

## Example: Integration with Applications

Similar to the `search-app` example in this repository, you can configure your application to use MySQL:

```yaml
deployments:
  - name: my-app
    image:
      repository: myapp/backend
      tag: "latest"
    container:
      env:
        - name: DATABASE_HOST
          value: "mysql.mysql.svc.cluster.local"
        - name: DATABASE_USERNAME
          valueFrom:
            secretKeyRef:
              name: mysql-secrets
              key: MYSQL_USER
        - name: DATABASE_PASSWORD
          valueFrom:
            secretKeyRef:
              name: mysql-secrets
              key: MYSQL_PASSWORD
```
