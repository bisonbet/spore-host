# Truffle

Truffle finds and compares EC2 instance types. It's read-only — it never launches anything. Use it to research what's available before committing to a launch.

## Install

```sh
brew install spore-host/tap/truffle
```

## Sub-commands

Truffle has distinct sub-commands for different tasks. They are **not interchangeable** — flags available on one command may not exist on another.

### `truffle find` — natural language search

Discover instance families using plain language. Understands processor names, GPU models, network capabilities, and size descriptions.

```sh
truffle find "epyc genoa"           # AMD EPYC Genoa (4th gen)
truffle find "h100 8gpu efa"        # NVIDIA H100 with EFA networking
truffle find "graviton large"       # ARM64 Graviton, large size class
truffle find "sapphire rapids 32 cores"
truffle find "milan 64gb"
```

Include specs **in the query string** — `truffle find` does not accept `--min-vcpu` or `--min-memory`:
```sh
truffle find "epyc genoa 16 cores"      # ✅ spec in query
truffle find "epyc genoa" --min-vcpu 16 # ❌ --min-vcpu not available on find
```

Flags:
- `--skip-azs` — faster, skip AZ lookup
- `--regions` — limit to specific regions
- `--app <name>` — find instances suitable for a catalog application

### `truffle search` — pattern search with filters

Search by instance type name pattern (wildcards and regex). Supports numeric filters.

```sh
truffle search "m8a.*"                              # all m8a sizes
truffle search "m8a.*" --min-vcpu 16               # ✅ --min-vcpu works here
truffle search "m8a.*" --min-vcpu 16 --min-memory 64
truffle search "c7a.*" --architecture x86_64
truffle search "g5.*" --skip-azs
```

The pattern is anchored — it must match the full instance type name. Wildcards (`*`, `?`) are supported.

Flags: `--min-vcpu`, `--min-memory`, `--architecture`, `--family`, `--show-price`, `--pick-first`, `--skip-azs`

### `truffle spot` — current Spot prices

Get live Spot prices for a specific instance type across regions and AZs.

```sh
truffle spot m8a.4xlarge
truffle spot "m7a.*" --sort-by-price --active-only
truffle spot g5.xlarge --regions us-east-1,us-west-2 --show-savings
```

### `truffle quotas` — service quota check

Check vCPU quotas before launching to avoid capacity errors.

```sh
truffle quotas --regions us-east-1
truffle quotas --family Standard --regions us-east-1   # M, C, R, T instances
truffle quotas --family P --regions us-east-1          # P-family GPU instances
truffle quotas --service sagemaker --family g5         # SageMaker ml.g5.* quotas
truffle quotas --family Standard --request             # generate increase commands
```

**Instance family codes:**

| Code | Instances |
|------|-----------|
| `Standard` | A, C, D, H, I, M, R, T, Z (general purpose) |
| `G` | g4dn, g5, g6 (graphics/GPU) |
| `P` | p3, p4, p5 (GPU training) |
| `Inf` | inf1, inf2 (Inferentia) |
| `Trn` | trn1 (Trainium) |

### `truffle capacity` — capacity reservations

Check existing On-Demand Capacity Reservations in your account.

```sh
truffle capacity
truffle capacity --gpu-only
truffle capacity --instance-types p5.48xlarge,p4d.24xlarge
```

## Typical workflow: find → search → spot → check quota → launch

```sh
# 1. Discover the instance family
truffle find "epyc genoa"

# 2. Browse sizes within that family (with spec filters)
truffle search "m8a.*" --min-vcpu 16 --min-memory 64

# 3. Check current Spot prices
truffle spot m8a.4xlarge --sort-by-price --active-only

# 4. Verify you have quota (m8a is Standard family)
truffle quotas --family Standard --regions us-east-1

# 5. Launch
spawn launch my-job --instance-type m8a.4xlarge --spot --ttl 4h
```

## Piping to spawn

Use `--pick-first` to get a single instance type name for piping:

```sh
spawn launch my-job \
  --instance-type $(truffle search "m8a.*" --min-vcpu 16 --pick-first) \
  --spot --ttl 4h
```

## Full command reference

→ [truffle command reference](/tools/reference/truffle)
