# spore.host AMI Build Framework

Templatized Packer build system for NICE DCV application streaming AMIs. Adding a new application is writing one ~10-line YAML recipe file.

## Architecture

```
infra/amis/
  base/
    dcv-cpu-al2.pkr.hcl     ← CPU base: AL2 + DCV + spored (no GPU)
    dcv-gpu-al2.pkr.hcl     ← GPU base: AL2 + DCV + NVIDIA drivers + spored
    nvidia-versions.yaml    ← locked driver matrix (g4dn, g5, g6, g6e)
  apps/
    paraview.yaml           ← per-app recipe
    igv.yaml
    chimerax.yaml
    ...
  build.sh                  ← build driver
  catalog-update.sh         ← patch pkg/catalog/catalog.yaml with built AMI IDs
```

## Prerequisites

1. **Packer** installed: https://developer.hashicorp.com/packer/install
2. **AWS credentials** for infra account (812107987990) with EC2/AMI permissions
3. **Marketplace subscriptions** accepted for the NICE DCV AMIs:
   - CPU variant: search "NICE DCV" in AWS Marketplace → AWS owner
   - GPU variant: same, GPU/NVIDIA variant

## Building

### Step 1: Build base layers (once)

```sh
# CPU base (for IGV, QGIS, Fiji, DS9, etc.)
./build.sh --base dcv-cpu-al2 us-east-1

# GPU base — 535-series drivers for g6/g6e (L4/L40S)
packer build \
  -var "region=us-east-1" \
  -var "instance_type=g6.xlarge" \
  -var "driver_version=535.129.03" \
  -var "cuda_version=12.2" \
  -var "layer_name=dcv-gpu-al2-535" \
  base/dcv-gpu-al2.pkr.hcl

# GPU base — 525-series drivers for g4dn/g5 (T4/A10G)
packer build \
  -var "region=us-east-1" \
  -var "instance_type=g5.xlarge" \
  -var "driver_version=525.105.17" \
  -var "cuda_version=12.0" \
  -var "layer_name=dcv-gpu-al2-525" \
  base/dcv-gpu-al2.pkr.hcl
```

### Step 2: Build an app AMI

```sh
# Single app, single region
./build.sh paraview us-east-1

# Single app, all regions
SPORE_BUILD_REGIONS=us-east-1,us-west-2,eu-west-1 ./build.sh paraview

# All apps (long — plan for several hours)
./build.sh --all
```

### Step 3: Update the Go catalog

After a build, Packer writes `packer-manifest-<app>-<region>.json`. Patch the catalog:

```sh
./catalog-update.sh paraview
# or
./catalog-update.sh --all
```

This updates `pkg/catalog/catalog.yaml` with the new AMI IDs. Commit the result.

## Adding a new application

Create `apps/<appname>.yaml`:

```yaml
name: myapp
base: dcv-cpu-al2          # or dcv-gpu-al2-535 for GPU apps
regions: [us-east-1, us-west-2, eu-west-1]
catalog_name: myapp        # must match name in pkg/catalog/catalog.yaml

install: |
  yum install -y <dependencies>
  curl -fsSL <download-url> -o /tmp/myapp.tar.gz
  tar -xzf /tmp/myapp.tar.gz -C /opt/
  ln -sf /opt/myapp/bin/myapp /usr/local/bin/myapp

test: myapp --version
launch_command: /usr/local/bin/myapp
```

Also add an entry to `pkg/catalog/catalog.yaml` with `amis: {}`.

## NVIDIA driver matrix

See `base/nvidia-versions.yaml`. Update deliberately — a bad driver breaks GPU
rendering for all apps built on that base layer.

| Family | GPU | Driver | CUDA | Base layer |
|--------|-----|--------|------|------------|
| g4dn | T4 | 525.105.17 | 12.0 | dcv-gpu-al2-525 |
| g5 | A10G | 525.105.17 | 12.0 | dcv-gpu-al2-525 |
| g6 | L4 | 535.129.03 | 12.2 | dcv-gpu-al2-535 |
| g6e | L40S | 535.129.03 | 12.2 | dcv-gpu-al2-535 |

## Publishing to AWS Marketplace

See issue #286 for the full publishing process. Once an AMI is built and tested:

1. Copy it to all required regions (AWS Console or `aws ec2 copy-image`)
2. Submit for Marketplace listing (spore-host publisher account)
3. Update `pkg/catalog/catalog.yaml` with final Marketplace AMI IDs
