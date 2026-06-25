#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

export PATH="/usr/local/go/bin:${PATH:-}"

BRANCH="${1:-main}"
FORCE_APPS="${DEPLOY_FORCE_APPS:-0}"

OLD_SHA="$(git rev-parse HEAD)"
log() {
	echo "$*" >&2
}

log "Fetching and updating ${BRANCH} (was ${OLD_SHA:0:7})..."
git fetch origin "$BRANCH"
git checkout "$BRANCH"
git pull origin "$BRANCH"
NEW_SHA="$(git rev-parse HEAD)"
log "Now at ${NEW_SHA:0:7}"

DETERMINE_ARGS=("$OLD_SHA" "$NEW_SHA")
if [[ "$FORCE_APPS" == "1" ]]; then
	DETERMINE_ARGS=(--force-apps "${DETERMINE_ARGS[@]}")
fi

eval "$(bash ./.github/scripts/determine-deploy-targets.sh "${DETERMINE_ARGS[@]}")"

log "Building tools..."
make tools

for app in $DEPLOY_APPS; do
	log "Building ${app}..."
	make "$app"
done

for app in $DEPLOY_APPS; do
	if systemctl list-unit-files "${app}.service" --no-legend 2>/dev/null | grep -q "${app}.service"; then
		log "Restarting ${app}..."
		sudo systemctl restart "${app}"
	else
		log "Skipping ${app} restart (no systemd unit)"
	fi
done

log "Deploy complete (${NEW_SHA:0:7})"
