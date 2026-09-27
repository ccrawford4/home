#!/usr/bin/env bash
set -euo pipefail

PROVIDER="${1:-}"

if [[ "$PROVIDER" != "--ollama" && "$PROVIDER" != "--hailo" ]]; then
  echo "Usage: $0 --ollama | --hailo"
  exit 1
fi

if [[ "$PROVIDER" == "--ollama" ]]; then
  URL="https://ollama.calum.sh/api/chat"
  AUTH=""
else
  URL="https://ai.calum.sh/api/chat"
  AUTH="-H \"Authorization: Bearer ${TOKEN}\""
fi

PROMPTS=(
  "Explain how Kubernetes scheduling works in detail."
  "Explain how Linux processes and threads work in detail."
  "Explain how a transformer-based language model generates tokens in detail."
)

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "=========================================="
echo "Provider: $PROVIDER"
echo "Running 3 requests concurrently"
echo "=========================================="
echo

START=$(python3 -c 'import time; print(time.time())')

for i in 0 1 2; do
  (
    START_REQ=$(python3 -c 'import time; print(time.time())')

    if [[ "$PROVIDER" == "--ollama" ]]; then
      curl -s "$URL" \
        -H 'Content-Type: application/json' \
        -d "{
          \"model\": \"qwen2.5-coder:1.5b\",
          \"stream\": false,
          \"messages\": [
            {\"role\": \"user\", \"content\": \"${PROMPTS[$i]}\"}
          ],
          \"options\": {
            \"temperature\": 0,
            \"num_predict\": 300
          }
        }" > "$TMPDIR/$i.json"
    else
      curl -s "$URL" \
        -H 'Content-Type: application/json' \
        -H "Authorization: Bearer $TOKEN" \
        -d "{
          \"model\": \"qwen2.5-coder:1.5b\",
          \"stream\": false,
          \"messages\": [
            {\"role\": \"user\", \"content\": \"${PROMPTS[$i]}\"}
          ],
          \"options\": {
            \"temperature\": 0,
            \"num_predict\": 300
          }
        }" > "$TMPDIR/$i.json"
    fi

    END_REQ=$(python3 -c 'import time; print(time.time())')

    python3 - "$i" "$START_REQ" "$END_REQ" "$TMPDIR/$i.json" <<'PY'
import json
import sys

i = sys.argv[1]
start = float(sys.argv[2])
end = float(sys.argv[3])
path = sys.argv[4]

with open(path) as f:
    data = json.load(f)

elapsed = end - start
tokens = data.get("eval_count", 0)

print(
    f"Request {int(i)+1}: "
    f"{tokens} tokens | "
    f"{elapsed:.2f}s | "
    f"{tokens/elapsed:.2f} tok/s"
)
PY
  ) &
done

wait

END=$(python3 -c 'import time; print(time.time())')

echo
echo "=========================================="
echo "Aggregate wall time: $(python3 -c "print(f'{float(\"$END\") - float(\"$START\"):.2f}s')")"
echo "=========================================="
