# OpenID Proxy

Small Go service that exposes Kubernetes OpenID discovery data over HTTP for local Workload Identity Federation experiments.

It reads the same Kubernetes API paths as:

```sh
kubectl get --raw /.well-known/openid-configuration
kubectl get --raw /openid/v1/jwks
```

## Endpoints

- `GET /healthz` returns `204`
- `GET /issuer` returns the issuer as plain text
- `GET /.well-known/openid-configuration` returns the Kubernetes discovery document
- `GET /openid/v1/jwks` returns the Kubernetes JWKS document

## Configuration

- `PORT`: listen port, default `8080`
- `KUBERNETES_API_URL`: Kubernetes API server URL. If unset, the service uses in-cluster `KUBERNETES_SERVICE_HOST` and `KUBERNETES_SERVICE_PORT`.

Kubernetes authentication uses the pod's mounted service account token and CA certificate from the standard in-cluster paths.

The service account needs permission to read the Kubernetes non-resource URLs:

```yaml
rules:
  - nonResourceURLs:
      - /.well-known/openid-configuration
      - /openid/v1/jwks
    verbs:
      - get
```
