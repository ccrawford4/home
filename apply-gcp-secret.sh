#!/bin/bash

# Usage: ./apply-gcp-secret.sh <project-id> <hostname>
# Example: ./apply-gcp-secret.sh my-project-123 pi@192.168.1.100

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <project-id> <hostname>"
    echo "Example: $0 my-project-123 pi@192.168.1.100"
    exit 1
fi

PROJECT_ID=$1
HOSTNAME=$2

echo "Creating service account key for project: $PROJECT_ID"

# Create the service account key and capture output
gcloud iam service-accounts keys create credentials \
    --iam-account=secrets-manager-sa@${PROJECT_ID}.iam.gserviceaccount.com

if [ $? -ne 0 ]; then
    echo "Failed to create service account key"
    exit 1
fi

echo "Reading credentials file..."
CREDENTIALS=$(cat credentials)

# Create the secret YAML with proper indentation
cat > gcp-sa-secret.yaml << EOF
apiVersion: v1
kind: Secret
metadata:
  name: gcp-sa-secret
type: Opaque
stringData:
  secret-access-credentials: |-
$(echo "$CREDENTIALS" | sed 's/^/    /')
EOF

echo "Generated gcp-sa-secret.yaml"

# Copy the secret file to the remote host
echo "Copying secret to $HOSTNAME..."
scp gcp-sa-secret.yaml ${HOSTNAME}:~/gcp-sa-secret.yaml

if [ $? -ne 0 ]; then
    echo "Failed to copy file to remote host"
    exit 1
fi

# Clean up local credentials file
rm credentials
rm gcp-sa-secret.yaml
