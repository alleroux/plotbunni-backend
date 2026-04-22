#!/usr/bin/env bash
# Usage: ./scripts/deploy.sh <stage|prod> <backend-tag> <frontend-tag>
#
# Deploys to a remote host over SSH using docker compose.
# To swap providers, replace the ssh block below with your provider's CLI:
#   - Fly.io:   fly deploy --image ghcr.io/alleroux/plotbunni-backend:$BACKEND_TAG
#   - Railway:  railway redeploy
#   - Render:   curl -X POST $RENDER_DEPLOY_HOOK_URL
#   - AWS ECS:  aws ecs update-service --force-new-deployment ...
set -euo pipefail

ENV="${1:?Usage: deploy.sh <stage|prod> <backend-tag> <frontend-tag>}"
BACKEND_TAG="${2:?missing backend-tag}"
FRONTEND_TAG="${3:?missing frontend-tag}"

case "$ENV" in
  stage)
    SSH_HOST="${STAGE_SSH_HOST:?Set STAGE_SSH_HOST in your environment}"
    SSH_USER="${STAGE_SSH_USER:-deploy}"
    ENV_FILE="envs/stage.env"
    ;;
  prod)
    SSH_HOST="${PROD_SSH_HOST:?Set PROD_SSH_HOST in your environment}"
    SSH_USER="${PROD_SSH_USER:-deploy}"
    ENV_FILE="envs/prod.env"
    ;;
  *)
    echo "error: unknown environment '$ENV'" >&2
    exit 1
    ;;
esac

if [[ ! -f "$ENV_FILE" ]]; then
  echo "error: $ENV_FILE not found — copy envs/${ENV}.env.example and fill it in" >&2
  exit 1
fi

echo "==> Deploying backend=$BACKEND_TAG frontend=$FRONTEND_TAG to $ENV ($SSH_USER@$SSH_HOST)"

# Upload env file and compose config to the server
scp "$ENV_FILE" "$SSH_USER@$SSH_HOST:~/plotbunni/.env"
scp docker-compose.prod.yml "$SSH_USER@$SSH_HOST:~/plotbunni/docker-compose.prod.yml"

ssh "$SSH_USER@$SSH_HOST" bash -s -- "$BACKEND_TAG" "$FRONTEND_TAG" <<'REMOTE'
set -euo pipefail
BACKEND_TAG="$1"
FRONTEND_TAG="$2"
cd ~/plotbunni

echo "--> Pulling images..."
BACKEND_TAG="$BACKEND_TAG" FRONTEND_TAG="$FRONTEND_TAG" \
  docker compose -f docker-compose.prod.yml pull backend frontend

echo "--> Starting services..."
BACKEND_TAG="$BACKEND_TAG" FRONTEND_TAG="$FRONTEND_TAG" \
  docker compose -f docker-compose.prod.yml up -d

echo "--> Health check..."
sleep 5
if curl -fsS http://localhost:8080/health > /dev/null; then
  echo "Deploy complete."
else
  echo "Health check failed — check 'docker compose -f docker-compose.prod.yml logs backend'" >&2
  exit 1
fi
REMOTE
