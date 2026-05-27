# Finding the Right Instance

This tutorial walks through the complete workflow for discovering and evaluating EC2 instance types before launching — using `truffle find`, `truffle search`, `truffle spot`, and `truffle quotas` in sequence.

## The workflow

```
truffle find    →  discover the instance family
truffle search  →  browse sizes and filter by specs
truffle spot    →  check current Spot prices
truffle quotas  →  verify you have headroom
spawn launch    →  launch with confidence
```

Each step is a separate sub-command. Flags like `--min-vcpu` only exist on `truffle search`, not on `truffle find`.

---

## Example: AMD EPYC Genoa instances

### Step 1 — Discover the family with `truffle find`

Use plain language to discover which instance families match a processor or capability:

```sh
truffle find "epyc genoa"
```

Output (abbreviated):
```
Found 40 instance types across 11 region(s)

┌────────────────┬────────────────┬───────┬──────────────┬──────────────┐
│ Instance Type  │ Region         │ vCPUs │ Memory (GiB) │ Architecture │
├────────────────┼────────────────┼───────┼──────────────┼──────────────┤
│ hpc7a.48xlarge │ us-east-2      │ 96    │ 768.0        │ x86_64       │
│ m7a.medium     │ eu-central-1   │ 1     │ 4.0          │ x86_64       │
│ m7a.large      │ eu-central-1   │ 2     │ 8.0          │ x86_64       │
│ m8a.4xlarge    │ us-east-1      │ 16    │ 64.0         │ x86_64       │
│ c7a.xlarge     │ us-east-1      │ 4     │ 8.0          │ x86_64       │
...
```

You can see the EPYC Genoa families: `m7a`, `m8a`, `c7a`, `r7a`, `hpc7a`. To include spec requirements in the query, put them in the string:

```sh
truffle find "epyc genoa 16 cores"
truffle find "epyc genoa 64gb"
```

::: tip find vs search flags
`truffle find` does not accept `--min-vcpu` or `--min-memory`. Include specs in the query string instead. Use `truffle search` when you want to filter by exact numeric thresholds.
:::

---

### Step 2 — Browse sizes with `truffle search`

Once you know the family, use `truffle search` to browse all sizes with numeric filters:

```sh
truffle search "m8a.*" --min-vcpu 16 --min-memory 64 --skip-azs
```

Output:
```
Found 5 instance types across 3 region(s)

┌───────────────┬───────────┬───────┬──────────────┬──────────────┐
│ Instance Type │ Region    │ vCPUs │ Memory (GiB) │ Architecture │
├───────────────┼───────────┼───────┼──────────────┼──────────────┤
│ m8a.4xlarge   │ us-east-1 │ 16    │ 64.0         │ x86_64       │
│ m8a.8xlarge   │ us-east-1 │ 32    │ 128.0        │ x86_64       │
│ m8a.16xlarge  │ us-east-1 │ 64    │ 256.0        │ x86_64       │
...
```

Common `truffle search` flags:
- `--min-vcpu N` — minimum vCPU count
- `--min-memory N` — minimum memory in GiB
- `--architecture arm64|x86_64` — filter by CPU architecture
- `--show-price` — include on-demand pricing
- `--pick-first` — output only the top result (for scripting)

---

### Step 3 — Check Spot prices with `truffle spot`

Before committing, check current Spot prices. Spot can be 60–90% cheaper than on-demand:

```sh
truffle spot m8a.4xlarge --sort-by-price --active-only
```

Output:
```
┌───────────────┬───────────┬───────────────────┬───────────────┬───────────┬─────────┐
│ Instance Type │ Region    │ Availability Zone │ Spot Price/hr │ On-Demand │ Savings │
├───────────────┼───────────┼───────────────────┼───────────────┼───────────┼─────────┤
│ m8a.4xlarge   │ us-east-1 │ us-east-1a        │ $0.0812       │ $0.7750   │ 89%     │
│               │ us-east-1 │ us-east-1c        │ $0.0834       │ $0.7750   │ 89%     │
│               │ us-west-2 │ us-west-2b        │ $0.0901       │ $0.7750   │ 88%     │
```

To compare across regions, add `--regions`:
```sh
truffle spot m8a.4xlarge --regions us-east-1,us-east-2,eu-west-1 --sort-by-price
```

---

### Step 4 — Check your quota with `truffle quotas`

Every AWS account has vCPU quotas per instance family. `m8a` is in the **Standard** family (same quota as M, C, R, T instances):

```sh
truffle quotas --family Standard --regions us-east-1
```

Output:
```
┌──────────┬───────────┬────────────┬───────────┬────────────┬────────┐
│ Family   │ Type      │ Quota      │ In Use    │ Available  │ Status │
├──────────┼───────────┼────────────┼───────────┼────────────┼────────┤
│ Standard │ On-Demand │ 1480 vCPUs │ 153 vCPUs │ 1327 vCPUs │ ✅ OK  │
│ Standard │ Spot      │ 640 vCPUs  │ -         │ 640 vCPUs  │ ✅ OK  │
```

An `m8a.4xlarge` has 16 vCPUs. With 1327 available, you can run up to ~82 simultaneous on-demand instances before hitting quota.

If you're near the limit, generate a quota increase request:
```sh
truffle quotas --family Standard --regions us-east-1 --request
```

---

### Step 5 — Launch

```sh
spawn launch my-job \
  --instance-type m8a.4xlarge \
  --region us-east-1 \
  --spot \
  --ttl 4h
```

Or use `--pick-first` to pipe directly from `truffle search`:
```sh
spawn launch my-job \
  --instance-type $(truffle search "m8a.*" --min-vcpu 16 --pick-first) \
  --spot \
  --ttl 4h
```

---

## GPU example: NVIDIA H100

The same workflow applies for specialized instances:

```sh
# 1. Find H100 instances
truffle find "h100 efa"

# 2. Browse p5 sizes (H100 is in p5 family)
truffle search "p5.*" --skip-azs

# 3. Check Spot prices (p5 is rarely Spot — check on-demand)
truffle spot p5.48xlarge --regions us-east-1,us-west-2

# 4. Check P-family quota (separate from Standard)
truffle quotas --family P --regions us-east-1

# 5. Launch (p5 typically on-demand, not Spot)
spawn launch gpu-job --instance-type p5.48xlarge --ttl 24h
```

---

## Quick reference: which command for what

| Task | Command |
|------|---------|
| "What instances have H100 GPUs?" | `truffle find "h100"` |
| "What are all c7a sizes?" | `truffle search "c7a.*"` |
| "c7a instances with ≥32 vCPUs" | `truffle search "c7a.*" --min-vcpu 32` |
| "Current Spot price for g5.xlarge" | `truffle spot g5.xlarge` |
| "Do I have quota for more G instances?" | `truffle quotas --family G` |
| "Check capacity reservations" | `truffle capacity --gpu-only` |

## See also

- [Spot Instances guide](/guides/spot-instances) — handling interruptions, diversifying instance types
- [truffle command reference](/tools/reference/truffle) — all flags for every sub-command
