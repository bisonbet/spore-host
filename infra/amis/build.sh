#!/usr/bin/env bash
# build.sh — build a spore.host DCV application AMI from a recipe
#
# Usage:
#   ./build.sh paraview                     # build paraview in us-east-1
#   ./build.sh paraview us-west-2           # build in a specific region
#   ./build.sh --all                        # build all apps in all regions
#   ./build.sh --base dcv-cpu-al2          # build a base layer only
#
# Prerequisites:
#   - packer installed: https://developer.hashicorp.com/packer/install
#   - AWS credentials set (infra account 812107987990)
#   - Base AMIs already built (run --base first if needed)
#
# Environment:
#   SPORE_BUILD_REGIONS   comma-separated region list (default: us-east-1,us-west-2,eu-west-1)
#   SPORE_BUILD_DRYRUN    set to "true" to validate Packer templates without building

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APPS_DIR="${SCRIPT_DIR}/apps"
BASE_DIR="${SCRIPT_DIR}/base"
DEFAULT_REGIONS="${SPORE_BUILD_REGIONS:-us-east-1,us-west-2,eu-west-1}"
DRYRUN="${SPORE_BUILD_DRYRUN:-false}"

usage() {
  echo "Usage: $0 <app-name> [region]"
  echo "       $0 --all [region]"
  echo "       $0 --base <dcv-cpu-al2|dcv-gpu-al2>"
  exit 1
}

build_app() {
  local app="$1"
  local region="${2:-us-east-1}"
  local recipe="${APPS_DIR}/${app}.yaml"

  if [[ ! -f "$recipe" ]]; then
    echo "ERROR: No recipe found for '${app}' at ${recipe}" >&2
    echo "Available apps: $(ls "${APPS_DIR}"/*.yaml | xargs -n1 basename | sed 's/.yaml//' | tr '\n' ' ')" >&2
    exit 1
  fi

  # Parse recipe fields
  local base
  base=$(grep '^base:' "$recipe" | awk '{print $2}')

  echo "==> Building ${app} on ${base} in ${region}"

  # Generate a temporary Packer HCL from the recipe
  local install_script
  install_script=$(python3 -c "
import yaml, sys
with open(sys.argv[1]) as f:
    r = yaml.safe_load(f)
print(r.get('install', ''))
" "$recipe")

  local launch_cmd
  launch_cmd=$(grep '^launch_command:' "$recipe" | awk '{print $2}')

  local test_cmd
  test_cmd=$(grep '^test:' "$recipe" | sed 's/^test: //')

  local tmp_pkr
  tmp_pkr="$(mktemp /tmp/spore-build-XXXXXX.pkr.hcl)"
  trap "rm -f ${tmp_pkr}" EXIT

  cat > "$tmp_pkr" << PACKER_EOF
packer {
  required_plugins {
    amazon = {
      version = ">= 1.3.0"
      source  = "github.com/hashicorp/amazon"
    }
  }
}

# Look up the most recent spore base AMI in this region
data "amazon-ami" "base" {
  filters = {
    name             = "spore-${base}-*"
    state            = "available"
  }
  most_recent = true
  owners      = ["\${env("AWS_ACCOUNT_ID")}"]
  region      = "${region}"
}

source "amazon-ebs" "app" {
  region        = "${region}"
  source_ami    = data.amazon-ami.base.id
  instance_type = "c7i.xlarge"
  ssh_username  = "ec2-user"
  ami_name      = "spore-app-${app}-{{timestamp}}"
  ami_description = "spore.host ${app} — DCV application streaming"
  tags = {
    "spore:app"        = "${app}"
    "spore:base"       = "${base}"
    "spore:managed"    = "true"
    "spore:build-date" = "{{timestamp}}"
  }
  launch_block_device_mappings {
    device_name           = "/dev/xvda"
    volume_size           = 40
    volume_type           = "gp3"
    delete_on_termination = true
  }
}

build {
  name    = "spore-${app}"
  sources = ["source.amazon-ebs.app"]

  provisioner "shell" {
    inline = [
      $(echo "$install_script" | while IFS= read -r line; do echo "\"${line//\"/\\\"}\","; done)
    ]
  }

  provisioner "shell" {
    inline = [
      # Configure DCV to auto-launch this app on session creation
      "echo '[session-management/automatic-console-session]' | sudo tee -a /etc/dcv/dcv.conf",
      "echo 'storage-root = \"%home%\"' | sudo tee -a /etc/dcv/dcv.conf",
      "sudo sed -i 's|#owner = \"\"||' /etc/dcv/dcv.conf || true",
    ]
  }

  post-processor "manifest" {
    output     = "${SCRIPT_DIR}/packer-manifest-${app}-${region}.json"
    strip_path = true
  }
}
PACKER_EOF

  if [[ "$DRYRUN" == "true" ]]; then
    echo "==> Dry run: validating ${tmp_pkr}"
    packer validate "$tmp_pkr"
  else
    packer build "$tmp_pkr"
    echo "==> Built ${app} in ${region}. Run ./catalog-update.sh ${app} to update catalog.yaml"
  fi
}

build_base() {
  local layer="$1"
  local region="${2:-us-east-1}"
  echo "==> Building base layer ${layer} in ${region}"
  if [[ "$DRYRUN" == "true" ]]; then
    packer validate -var "region=${region}" "${BASE_DIR}/${layer}.pkr.hcl"
  else
    packer build -var "region=${region}" "${BASE_DIR}/${layer}.pkr.hcl"
  fi
}

# Parse arguments
case "${1:-}" in
  --all)
    region="${2:-}"
    for recipe in "${APPS_DIR}"/*.yaml; do
      app=$(basename "$recipe" .yaml)
      if [[ -n "$region" ]]; then
        build_app "$app" "$region"
      else
        IFS=',' read -ra regions <<< "$DEFAULT_REGIONS"
        for r in "${regions[@]}"; do
          build_app "$app" "$r"
        done
      fi
    done
    ;;
  --base)
    layer="${2:-}"
    region="${3:-us-east-1}"
    [[ -z "$layer" ]] && usage
    build_base "$layer" "$region"
    ;;
  "")
    usage
    ;;
  *)
    app="$1"
    region="${2:-us-east-1}"
    build_app "$app" "$region"
    ;;
esac
