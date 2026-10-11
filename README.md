# home

This repository is the control plane for a Raspberry Pi K3s home cluster. It
contains the Terraform, Helm charts, GitOps manifests, and small Go utilities
that make the cluster reproducible.

The interesting part is not just that it runs a few apps. The repo wires
together:

- K3s on Raspberry Pis as the Kubernetes runtime.
- Argo CD ApplicationSets for GitOps deployment.
- Cloudflare Tunnel and DNS for public hostnames.
- External Secrets Operator backed by Google Secret Manager.
- Google Workload Identity Federation (WIF) using Kubernetes service account
  tokens and a public OpenID issuer.
- A custom kubelet credential provider that exchanges projected Kubernetes
  service account tokens for Google Artifact Registry credentials.
- Reusable local Helm charts for applications, MySQL, and Redis.

## Architecture at a Glance

```mermaid
flowchart TD
  Dev[Engineer pushes to GitHub] --> Repo[ccrawford4/home]
  Repo --> AppSet[Argo CD ApplicationSet]
  AppSet --> Helm[helm/* charts]
  Helm --> Apps[Application namespaces]

  Apps --> ESO[External Secrets Operator]
  ESO --> Store[Per-namespace SecretStore]
  Store --> GSM[Google Secret Manager]

  Internet[Public Internet] --> CF[Cloudflare Tunnel + DNS]
  CF --> Traefik[Traefik Ingress in K3s]
  Traefik --> Apps

  K3s[K3s API server] --> OIDC[openid-server]
  OIDC --> CF
  GoogleSTS[Google STS] --> OIDC

  Kubelet[K3s kubelet] --> Provider[gar-credential-provider]
  Provider --> GoogleSTS
  Provider --> IAM[IAM Credentials API]
  IAM --> GAR[Google Artifact Registry]
  Kubelet --> GAR
```

## Repository Map

| Path | Purpose |
| --- | --- |
| `applicationset.yaml` | Argo CD `ApplicationSet` that discovers every chart under `helm/*` and syncs it. |
| `install.sh` | Installs Argo CD into the cluster with the server exposed as an insecure in-cluster service for Traefik/Cloudflare. |
| `terraform/` | GCP, Cloudflare, Secret Manager, Artifact Registry, and WIF infrastructure. |
| `helm/` | Deployable cluster charts: namespaces, networking, identity, apps, and vendored External Secrets Operator. |
| `helm-library/` | Local reusable charts for app deployments, MySQL, and Redis. |
| `infrastructure/k3s/` | K3s config needed for service-account issuer metadata and kubelet image credential providers. |
| `infrastructure/openid-server/` | Go service that exposes Kubernetes OpenID discovery and JWKS through a public hostname. |
| `infrastructure/gar-credential-provider/` | Go kubelet credential-provider plugin for pulling private GAR images with WIF. |

## Deployed Components

The top-level GitOps loop is simple:

```mermaid
sequenceDiagram
  participant Git as GitHub repo
  participant Argo as Argo CD
  participant Helm as Helm chart
  participant K8s as K3s cluster

  Git->>Argo: applicationset.yaml points at helm/*
  Argo->>Git: watches HEAD
  Argo->>Helm: renders each helm/* chart
  Helm->>K8s: applies resources
  Argo->>K8s: prunes drift and self-heals
```

Current first-class charts:

| Chart | What it deploys |
| --- | --- |
| `helm/namespaces` | Cluster namespaces used by the apps and infrastructure charts. |
| `helm/external-secrets` | External Secrets Operator chart, including CRDs. |
| `helm/networking` | Traefik `Ingress` resources for `search.calum.sh`, `about.calum.sh`, `argocd.calum.sh`, and `openid.calum.sh`. |
| `helm/search-app` | Search frontend, search backend, MySQL, Redis, and synced app secrets. |
| `helm/portfolio` | Portfolio app and an example private GAR-backed nginx deployment. |
| `helm/ai-agent-api` | API with Redis plus Kubernetes read permissions. |
| `helm/openid-server` | Public OpenID proxy used by the WIF flow. |

## Terraform

Terraform owns the cloud-side resources:

- `workloads.tf`: one map entry per namespace listing its Secret Manager
  secrets. Adding a secret is a one-line change.
- `edge.tf`: one map entry per public hostname. Generates the Cloudflare Tunnel
  ingress rules, proxied DNS records and Zero Trust Access applications.
- `platform.tf`: Google Workload Identity Pool and OIDC provider, the private
  Artifact Registry repository `internal`, and GCS buckets.
- `service_accounts.tf`: `gar-puller`, the image-pull identity used by the
  kubelet credential provider.
- `modules/workload`: Secret Manager secrets for one namespace plus who may
  read them, and an optional dedicated Google service account
  (`service_account = { k8s_service_accounts = [...], project_roles = [...] }`).
- `modules/public_hostname`: DNS record and Access applications for one hostname.

The required local variables are shown in
`terraform/secrets.auto.tfvars.example`:

```hcl
project_id     = "<your gcloud project id>"
project_number = "<your gcloud project number>"
region         = "us-central1"
k8s_issuer_uri = "https://openid.example.com"

cloudflare_api_token     = "<cloudflare api token>"
cloudflare_account_id    = "<cloudflare account id>"
cloudflare_tunnel_secret = "<cloudflare tunnel secret>"
cloudflare_zone_id       = "<cloudflare zone id>"
k8s_server_ip            = "<private or tunnel-reachable K3s ingress IP>"
```

Run Terraform from the `terraform` directory:

```bash
cd terraform
terraform init
terraform plan
terraform apply
```

Terraform creates Secret Manager secret containers, not secret versions. After
apply, add values explicitly:

```bash
echo -n "actual-secret-value" | gcloud secrets versions add search-app-db-password \
  --project "$PROJECT_ID" \
  --data-file=-
```

Use `echo -n` so the secret does not accidentally include a trailing newline.

## Secrets Model

Google Secret Manager holds every secret. External Secrets Operator syncs them
into Kubernetes Secrets, and each namespace can only read its own.

```mermaid
flowchart LR
  GSM[Google Secret Manager] --> ESO[External Secrets Operator]
  KSA[App Kubernetes service account token] --> STS[Google STS]
  STS --> Store[SecretStore gcp-secret-manager in app namespace]
  Store --> ESO
  ESO --> K8sSecret[Kubernetes Secret in app namespace]
  K8sSecret --> Pod[Application pod env vars]
```

Terraform (`terraform/modules/workload`) creates the `<namespace>-*` secrets
and grants `roles/secretmanager.secretAccessor` on them to every Kubernetes
service account in that namespace:

```text
principalSet://iam.googleapis.com/projects/<project-number>/locations/global/workloadIdentityPools/<pool-id>/attribute.ns/<namespace>
```

The `application-template` chart creates a `SecretStore` named
`gcp-secret-manager` in the app namespace. It authenticates with Workload
Identity Federation as the app's own service account, so there are no Google
service account keys:

```yaml
spec:
  provider:
    gcpsm:
      projectID: "home-473419"
      auth:
        workloadIdentityFederation:
          audience: "//iam.googleapis.com/projects/<project-number>/locations/global/workloadIdentityPools/home-cluster-pool/providers/home-cluster-oidc-provider"
          serviceAccountRef:
            name: search-app
            audiences:
              - "//iam.googleapis.com/projects/<project-number>/locations/global/workloadIdentityPools/home-cluster-pool/providers/home-cluster-oidc-provider"
```

Application charts then define `ExternalSecret` resources through the
`secrets` list:

```yaml
secrets:
  - name: search-app-secrets
    targetName: search-app-secrets
    data:
      - secretKey: db-password
        remoteRefKey: search-app-db-password
```

The app consumes the synced Kubernetes secret like any normal environment
variable:

```yaml
env:
  - name: MYSQL_PASSWORD
    valueFrom:
      secretKeyRef:
        name: search-app-secrets
        key: db-password
```

The WIF provider in `terraform/platform.tf` maps the token claims used above:

```hcl
attribute_mapping = {
  "google.subject" = "assertion.sub"
  "attribute.ns"   = "assertion['kubernetes.io']['namespace']"
  "attribute.sa"   = "assertion['kubernetes.io']['serviceaccount']['name']"
}
```

## Kubernetes OpenID Issuer

Google WIF needs to fetch the issuer discovery document and JWKS for Kubernetes
service account tokens. K3s is configured in `infrastructure/k3s/config.yaml`:

```yaml
kube-apiserver-arg:
  - service-account-issuer=https://openid.calum.sh
  - service-account-jwks-uri=https://openid.calum.sh/openid/v1/jwks
```

The `openid-server` app exposes the Kubernetes API server's built-in discovery
endpoints:

```text
GET /.well-known/openid-configuration
GET /openid/v1/jwks
GET /issuer
GET /healthz
```

It reads the raw Kubernetes API paths:

```bash
kubectl get --raw /.well-known/openid-configuration
kubectl get --raw /openid/v1/jwks
```

Then it rewrites `issuer` and `jwks_uri` to the public issuer URL
(`https://openid.calum.sh`) so Google STS can validate tokens through the
Cloudflare-routed hostname.

The OpenID server service account needs non-resource URL access:

```yaml
rules:
  - nonResourceURLs:
      - "/.well-known/openid-configuration"
      - "/openid/v1/jwks"
    verbs: ["get"]
```

## Private Artifact Registry Pulls

The `gar-credential-provider` is a kubelet exec credential provider. It lets the
node pull private images from Google Artifact Registry using the pod's projected
Kubernetes service account token instead of a Docker config secret.

```mermaid
sequenceDiagram
  participant Kubelet
  participant Plugin as gar-credential-provider
  participant STS as Google STS
  participant IAM as IAM Credentials API
  participant GAR as Artifact Registry

  Kubelet->>Plugin: CredentialProviderRequest(image, serviceAccountToken)
  Plugin->>STS: exchange Kubernetes JWT for federated access token
  STS->>Plugin: STS access token
  Plugin->>IAM: generateAccessToken for gar-puller
  IAM->>Plugin: Google OAuth access token
  Plugin->>Kubelet: Docker auth for us-central1-docker.pkg.dev
  Kubelet->>GAR: pull private image
```

K3s must enable the kubelet feature gate and point to the provider config:

```yaml
kubelet-arg:
  - "image-credential-provider-config=/etc/rancher/k3s/credential-provider-config.yaml"
  - "image-credential-provider-bin-dir=/var/lib/rancher/gcp-credential-provider/bin"
  - "feature-gates=KubeletServiceAccountTokenForCredentialProviders=true"
```

`infrastructure/k3s/credential-provider-config.yaml` matches the internal GAR
repository and passes the WIF audience:

```yaml
matchImages:
  - "us-central1-docker.pkg.dev/home-473419/internal"
env:
  - name: GAR_IMAGE_PREFIX
    value: us-central1-docker.pkg.dev/home-473419/internal
  - name: STS_AUDIENCE
    value: "//iam.googleapis.com/projects/631401797177/locations/global/workloadIdentityPools/home-cluster-pool/providers/home-cluster-oidc-provider"
  - name: SERVICE_ACCOUNT_EMAIL
    value: gar-puller@home-473419.iam.gserviceaccount.com
tokenAttributes:
  serviceAccountTokenAudience: "//iam.googleapis.com/projects/631401797177/locations/global/workloadIdentityPools/home-cluster-pool/providers/home-cluster-oidc-provider"
  requireServiceAccount: true
```

The plugin validates the image prefix, exchanges the service account JWT with
Google STS, impersonates the configured Google service account, and returns a
`CredentialProviderResponse` containing Docker auth.

`gar-puller` only has `roles/artifactregistry.reader` on the `internal`
repository. A pod may use it only if its Kubernetes service account is listed
under `image_pull_service_accounts` for its namespace in
`terraform/workloads.tf`:

```hcl
virgo = {
  secrets                     = ["webui-admin-password"]
  image_pull_service_accounts = ["virgo"]
}
```

Example response:

```json
{
  "kind": "CredentialProviderResponse",
  "cacheKeyType": "Image",
  "auth": {
    "us-central1-docker.pkg.dev": {
      "username": "oauth2accesstoken",
      "password": "<impersonated-access-token>"
    }
  }
}
```

## Helm Application Pattern

Application charts in `helm/*` are intentionally thin. They compose local
library charts:

```yaml
dependencies:
  - name: application-template
    version: 0.1.0
    repository: "file://../../helm-library/application-template"
  - name: redis
    version: 0.1.0
    repository: "file://../../helm-library/redis"
```

The `application-template` chart can render:

- `ServiceAccount`
- optional `ClusterRole` and `ClusterRoleBinding`
- `ConfigMap`
- `ExternalSecret` or static `Secret`
- `Deployment`
- `Service`
- `HorizontalPodAutoscaler`

A minimal app values file looks like:

```yaml
application-template:
  name: example
  namespace: example
  serviceAccount:
    create: true
    name: example
  deployments:
    - name: example
      replicaCount: 2
      image:
        repository: ghcr.io/example/app
        tag: "v1.0.0"
        port: 8080
      service:
        enabled: true
        port: 80
        targetPort: 8080
```

MySQL and Redis are also local dependency charts. They support persistent
volumes and either External Secrets or static Kubernetes secrets. See
`helm/search-app/values.yaml` for a complete app that composes all three
library charts.

## Getting Started

### 1. Prepare the cluster

Install K3s on the nodes, then apply the relevant settings from
`infrastructure/k3s/config.yaml`. For private GAR image pulls, also copy:

- `infrastructure/k3s/credential-provider-config.yaml` to
  `/etc/rancher/k3s/credential-provider-config.yaml`
- the compiled `gar-credential-provider` binary to the kubelet credential
  provider bin directory, for example
  `/var/lib/rancher/gcp-credential-provider/bin`

Restart K3s after changing kube-apiserver or kubelet args.

### 2. Create cloud infrastructure

Create `terraform/secrets.auto.tfvars` from the example, then run:

```bash
cd terraform
terraform init
terraform plan
terraform apply
```

At minimum, make sure these are correct for your environment:

- `project_id`
- `project_number`
- `k8s_issuer_uri`
- Cloudflare account, zone, tunnel, and API token values
- `k8s_server_ip`

The WIF issuer URL must be publicly reachable by Google STS before keyless token
exchange can work.

### 3. Add Secret Manager versions

Terraform creates secret resources for:

- `search-app`
- `ai-agent-api`
- `portfolio`
- `openid-server`

Add versions for every required secret:

```bash
gcloud secrets versions add openid-server-kubernetes-api-url \
  --project "$PROJECT_ID" \
  --data-file=<(printf %s "https://<kubernetes-api-server>")
```

If your shell does not support process substitution, write the value to a
temporary file and pass that file to `--data-file`.

### 4. Install Argo CD

From the cluster node or any machine with working `kubectl` and `helm` access:

```bash
./install.sh
kubectl apply -f applicationset.yaml
```

Argo CD will discover all directories under `helm/*` and reconcile them into the
cluster.

### 5. Check the rollout

Useful commands:

```bash
kubectl get applications -n argocd
kubectl get pods -A
kubectl get externalsecrets -A
kubectl get ingress -A
kubectl logs -n openid-server deploy/openid-server
```

For WIF/OpenID validation:

```bash
curl https://openid.calum.sh/.well-known/openid-configuration
curl https://openid.calum.sh/openid/v1/jwks
```

For private image pull debugging, inspect kubelet/provider logs and the provider
log configured in `credential-provider-config.yaml`:

```bash
sudo tail -f /var/log/gcp-credential-provider.log
```

## Adding a New Application

1. Create `helm/<app>/Chart.yaml`.

```yaml
apiVersion: v2
name: my-app
type: application
version: 0.1.0
dependencies:
  - name: application-template
    version: 0.1.0
    repository: "file://../../helm-library/application-template"
```

2. Create `helm/<app>/values.yaml`.

```yaml
application-template:
  name: my-app
  namespace: my-app
  serviceAccount:
    create: true
    name: my-app
  deployments:
    - name: my-app
      image:
        repository: ghcr.io/example/my-app
        tag: "v0.1.0"
        port: 8080
      service:
        enabled: true
        port: 80
        targetPort: 8080
```

3. Add the namespace in `helm/namespaces/values.yaml`.

```yaml
namespaces:
  - name: my-app
```

4. Add ingress in `helm/networking/values.yaml` if the app is public.

```yaml
namespaces:
  my-app:
    rules:
      - host: my-app.calum.sh
        paths:
          - path: /
            pathType: Prefix
            serviceName: my-app
            servicePort: 80
```

5. Add the app to `local.workloads` in `terraform/workloads.tf` if it needs
   secrets. Only service accounts in `my-app` can read them.

```hcl
my-app = {
  secrets = ["api-key"] # creates my-app-api-key
}
```

6. Reference those secrets from Helm.

```yaml
application-template:
  secrets:
    - name: my-app-secrets
      data:
        - secretKey: api-key
          remoteRefKey: my-app-api-key
  deployments:
    - name: my-app
      container:
        env:
          - name: API_KEY
            valueFrom:
              secretKeyRef:
                name: my-app-secrets
                key: api-key
```

Argo CD will pick up the new chart automatically because the ApplicationSet
generator watches `helm/*`.

## Local Development and Validation

Build or test the Go utilities directly:

```bash
cd infrastructure/openid-server
go test ./...
go build ./...

cd ../gar-credential-provider
go test ./...
go build ./...
```

Render a Helm chart before committing:

```bash
helm dependency update helm/search-app
helm template search-app helm/search-app
```

Run Terraform checks:

```bash
cd terraform
terraform fmt -check
terraform validate
terraform plan
```

## Operational Notes

- Do not commit service account keys, `.auto.tfvars`, Terraform state, or
  kubeconfigs.
- The OpenID issuer hostname must keep serving discovery and JWKS documents. WIF
  token exchange and private image pulls depend on it.
- Secret Manager secrets created by Terraform are empty until a version is
  added.
- External Secrets is keyless: each namespace's `SecretStore` uses WIF with
  the app's own service account. If a store reports an auth error, check that
  the namespace has an entry in `terraform/workloads.tf` and that the service
  account named in the store exists.
- `applicationset.yaml` syncs every chart under `helm/*`, so partially created
  chart directories can become Argo CD applications.
- The vendored `helm/external-secrets` chart is large because it includes CRDs
  and upstream templates.

## Why This Is Interesting

This repo demonstrates a full small-cluster platform rather than a collection of
one-off manifests. The cluster has a GitOps deployment loop, cloud-managed
secrets, public ingress, app-level chart reuse, cloud IAM federation, and
keyless private image pull mechanics. The custom pieces are intentionally narrow:
the OpenID proxy makes K3s service account identity publicly verifiable, and the
GAR credential provider turns that identity into short-lived registry
credentials at image-pull time.

# Virgo (AI Accelerator NPU Node)
Next steps:
```bash
code=500
description=Internal Server Error
stacktrace:
  - Expected::expect() failed with status=36. Failed to create VDevice
  - [ApiController]: Error processing request
  - Error processing request
```
Need to setup VDevice
