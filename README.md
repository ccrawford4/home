# home
Kubernetes configuration for a cluster running on some Rasberry PIs

## Prerequisites


## Creating the secrets

```bash
export ACCOUNT_ID=<your cloudflare account id>
export TUNNEL_NAME=<your cloudflare tunnel name>
export API_TOKEN=<your cloudflare api token>

kubectl create secret generic cloudflare-external-secret \
    --from-literal=account_id=$ACCOUNT_ID \
    --from-literal=tunnel_name=$TUNNEL_NAME \
    --from-literal=api_token=$API_TOKEN
```
