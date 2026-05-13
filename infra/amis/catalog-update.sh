#!/usr/bin/env bash
# catalog-update.sh — patch pkg/catalog/catalog.yaml with built AMI IDs
#
# After running build.sh, Packer writes a manifest JSON with the new AMI IDs.
# This script reads those manifests and patches the 'amis:' block in
# pkg/catalog/catalog.yaml so the Go package reflects the latest builds.
#
# Usage:
#   ./catalog-update.sh paraview              # update paraview AMIs from manifest
#   ./catalog-update.sh --all                 # update all apps from all manifests
#
# The manifest files are named packer-manifest-<app>-<region>.json
# and live in this directory (created by build.sh).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CATALOG_YAML="${SCRIPT_DIR}/../../pkg/catalog/catalog.yaml"

usage() {
  echo "Usage: $0 <app-name>"
  echo "       $0 --all"
  exit 1
}

update_app() {
  local app="$1"

  echo "==> Updating catalog for ${app}"

  # Find all manifest files for this app
  local manifests
  mapfile -t manifests < <(ls "${SCRIPT_DIR}/packer-manifest-${app}-"*.json 2>/dev/null)

  if [[ ${#manifests[@]} -eq 0 ]]; then
    echo "  No manifest files found for ${app}. Run build.sh ${app} first." >&2
    return 1
  fi

  # Build a YAML amis mapping
  local amis_yaml="    amis:"
  for manifest in "${manifests[@]}"; do
    # Extract region from filename: packer-manifest-<app>-<region>.json
    local region
    region=$(basename "$manifest" .json | sed "s/packer-manifest-${app}-//")

    # Extract AMI ID from manifest
    local ami_id
    ami_id=$(python3 -c "
import json, sys
with open(sys.argv[1]) as f:
    m = json.load(f)
builds = m.get('builds', [])
if builds:
    arts = builds[-1].get('artifact_id', '')
    # artifact_id format: region:ami-id
    for part in arts.split(','):
        r, a = part.split(':')
        if r.strip() == sys.argv[2]:
            print(a.strip())
            break
" "$manifest" "$region" 2>/dev/null || echo "")

    if [[ -n "$ami_id" ]]; then
      amis_yaml="${amis_yaml}
      ${region}: ${ami_id}"
      echo "  ${region}: ${ami_id}"
    fi
  done

  # Use Python to patch the YAML file (preserves formatting better than sed)
  python3 - "$app" "$amis_yaml" << 'PYEOF'
import sys, re

app = sys.argv[1]
amis_block = sys.argv[2]

with open("${CATALOG_YAML}") as f:
    content = f.read()

# Replace the amis: {} or amis:\n      key: val block for this app
# Strategy: find the app entry and replace its amis section
pattern = rf'(  - name: {re.escape(app)}.*?)(    amis:.*?)(\n  - |\Z)'
replacement = rf'\1{amis_block}\3'

new_content = re.sub(pattern, replacement, content, flags=re.DOTALL)

if new_content == content:
    print(f"  Warning: could not update amis for {app} — check catalog.yaml format")
else:
    with open("${CATALOG_YAML}", 'w') as f:
        f.write(new_content)
    print(f"  Updated {app} in catalog.yaml")
PYEOF
}

case "${1:-}" in
  --all)
    for manifest in "${SCRIPT_DIR}"/packer-manifest-*.json; do
      app=$(basename "$manifest" | sed 's/packer-manifest-//' | sed 's/-.*//')
      update_app "$app" || true
    done
    ;;
  "")
    usage
    ;;
  *)
    update_app "$1"
    ;;
esac
