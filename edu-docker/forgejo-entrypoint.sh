#!/bin/sh
set -e

GITEA_BIN=/app/gitea
SHARED_DIR=/shared
TOKEN_FILE="$SHARED_DIR/runner-token"

mkdir -p "$SHARED_DIR"

if ! "$GITEA_BIN" admin user list 2>/dev/null | awk 'NR>1 {print $2}' | grep -qx eduadmin; then
  echo ">>> Creating eduadmin user..."
  "$GITEA_BIN" admin user create \
    --admin \
    --username eduadmin \
    --password 'Password123!' \
    --email eduadmin@example.com \
    --must-change-password=false || echo ">>> admin user create failed, may already exist"
fi

echo ">>> Starting Forgejo web server in background..."
"$GITEA_BIN" web &
FORGEJO_PID=$!

trap 'echo ">>> Stopping Forgejo..."; kill -TERM $FORGEJO_PID 2>/dev/null; wait $FORGEJO_PID' TERM INT

echo ">>> Waiting for Forgejo to be ready on http://localhost:3000 ..."
until wget -qO- http://localhost:3000/api/v1/version >/dev/null 2>&1; do
  sleep 2
done
echo ">>> Forgejo is ready."

if [ ! -s "$TOKEN_FILE" ]; then
  echo ">>> Generating runner registration token..."
  "$GITEA_BIN" actions generate-runner-token > "$TOKEN_FILE.tmp"
  mv "$TOKEN_FILE.tmp" "$TOKEN_FILE"
  chmod 644 "$TOKEN_FILE"
  echo ">>> Token written to $TOKEN_FILE"
else
  echo ">>> Token file already exists at $TOKEN_FILE, skipping generation."
fi

wait $FORGEJO_PID
