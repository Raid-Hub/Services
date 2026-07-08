#!/usr/bin/env bash
# Decide which Services binaries need rebuilding between two git refs.
#
# Tools are always rebuilt. Apps (hermes, atlas, zeus) are rebuilt only when
# changed files touch their source tree or Go dependency closure.
#
# Usage:
#   determine-deploy-targets.sh [--force-apps] [BASE_REF] [HEAD_REF]
#
# Prints shell assignments to stdout:
#   DEPLOY_TOOLS=1
#   DEPLOY_APPS="hermes atlas"
#
# Logs analysis details to stderr.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

FORCE_APPS=0
POSITIONAL=()

while [[ $# -gt 0 ]]; do
	case "$1" in
	--force-apps)
		FORCE_APPS=1
		shift
		;;
	-*)
		echo "Unknown option: $1" >&2
		exit 1
		;;
	*)
		POSITIONAL+=("$1")
		shift
		;;
	esac
done

BASE_REF="${POSITIONAL[0]:-HEAD~1}"
HEAD_REF="${POSITIONAL[1]:-HEAD}"

ALL_APPS=(hermes atlas zeus)
DEPLOY_APPS=()

log() {
	echo "$*" >&2
}

app_dep_dirs() {
	local app="$1"
	go list -deps -f '{{.Dir}}' "./apps/${app}/" | grep "^${REPO_ROOT}/" || true
}

file_affects_app() {
	local file="$1"
	local app="$2"
	local abs="${REPO_ROOT}/${file}"

	[[ "$file" == apps/${app}/* ]] && return 0

	local dir
	while IFS= read -r dir; do
		[[ -z "$dir" ]] && continue
		if [[ "$abs" == "${dir}/"* ]] || [[ "$abs" == "$dir" ]]; then
			return 0
		fi
	done < <(app_dep_dirs "$app")

	return 1
}

rebuild_all_apps() {
	local reason="$1"
	log "Rebuilding all apps: ${reason}"
	DEPLOY_APPS=("${ALL_APPS[@]}")
}

if [[ "$FORCE_APPS" -eq 1 ]]; then
	rebuild_all_apps "forced"
elif [[ "$BASE_REF" == "0000000000000000000000000000000000000000" ]]; then
	rebuild_all_apps "no previous commit (force-push or first deploy)"
else
	CHANGED_FILES=()
	while IFS= read -r file; do
		[[ -n "$file" ]] && CHANGED_FILES+=("$file")
	done < <(git diff --name-only "$BASE_REF" "$HEAD_REF")

	if [[ ${#CHANGED_FILES[@]} -eq 0 ]]; then
		log "No changed files between ${BASE_REF} and ${HEAD_REF}"
	else
		log "Changed files (${#CHANGED_FILES[@]}):"
		for file in "${CHANGED_FILES[@]}"; do
			log "  - ${file}"
		done

		for file in "${CHANGED_FILES[@]}"; do
			case "$file" in
			go.mod | go.sum | Makefile)
				rebuild_all_apps "${file} changed"
				break
				;;
			esac
		done

		if [[ ${#DEPLOY_APPS[@]} -eq 0 ]]; then
			for app in "${ALL_APPS[@]}"; do
				for file in "${CHANGED_FILES[@]}"; do
					if file_affects_app "$file" "$app"; then
						log "${app}: affected by ${file}"
						DEPLOY_APPS+=("$app")
						break
					fi
				done
			done
		fi
	fi
fi

log "Deploy plan: tools=always apps=${DEPLOY_APPS[*]:-(none)}"

echo "DEPLOY_TOOLS=1"
if [[ ${#DEPLOY_APPS[@]} -eq 0 ]]; then
	echo 'DEPLOY_APPS=""'
else
	echo "DEPLOY_APPS=\"${DEPLOY_APPS[*]}\""
fi
