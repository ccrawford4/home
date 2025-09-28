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


## TODO

- Need to configure kube-apiserver so that it supports ServiceAccount token volume projections.
https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#serviceaccount-token-volume-projection
