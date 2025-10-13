# home
Kubernetes configuration for a cluster running on some Rasberry PIs

## Prerequisites
- A set of Raspberry Pis with K3s installed and running
- SSH access to the master node
- Helm installed on the master node

## Installation

1. SSH into the master node
```bash
# Or whatever your ssh command is
ssh pi@<master-node-ip>
```

2. Clone this repository

```bash
git clone https://github.com/ccrawford4/home.git
```

3. Navigate to the `home` directory and run the install script

```bash
cd home
./install.sh
```

4. Then add the applicationset

```bash
kubectl apply -f argocd-applicationset.yaml
```

5. Add a tunnel (I'm using cloudflare) so you can access it from outside your network


## Generating Secrets

### Create the kubernetes secret to authenticate with Google Cloud

#### Using the automated script (recommended)

1. Run the script with your GCP project ID and SSH hostname:
```bash
./apply-gcp-secret.sh <project-id> <hostname>
```
Example:
```bash
./apply-gcp-secret.sh home-473419 pi@192.168.1.100
```

2. SSH into your host to verify the secret was created, and then apply it to the cluster:
```bash
ssh <hostname>
kubectl apply -f gcp-sa-secret.yaml
```

This will:
1. Create the service account key
2. Generate the `gcp-sa-secret.yaml` with the credentials
3. Copy it to the remote host

#### Manual steps

If you prefer to do it manually:

1. Create the service account key
```bash
gcloud iam service-accounts keys create credentials \
    --iam-account=secrets-manager-sa@<project-id>.iam.gserviceaccount.com
```

2. Update `gcp-sa-secret.yaml` with the contents of the credentials file

3. Copy the secret to your cluster and apply it
```bash
scp gcp-sa-secret.yaml <hostname>:~/gcp-sa-secret.yaml
ssh <hostname> "kubectl apply -f gcp-sa-secret.yaml"
```
