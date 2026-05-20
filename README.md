<h1 align="center">🍄 spore.host</h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License: Apache 2.0"></a>
  <a href="https://github.com/spore-host/spore-host/actions/workflows/ci.yml"><img src="https://github.com/spore-host/spore-host/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/spore-host/spore-host"><img src="https://codecov.io/gh/spore-host/spore-host/branch/main/graph/badge.svg" alt="Coverage"></a>
  <a href="https://goreportcard.com/report/github.com/spore-host/spore-host/spawn"><img src="https://goreportcard.com/badge/github.com/spore-host/spore-host/spawn" alt="Go Report Card"></a>
  <a href="https://github.com/spore-host/spore-host/releases/latest"><img src="https://img.shields.io/github/v/release/spore-host/spore-host" alt="Latest Release"></a>
  <a href="https://img.shields.io/badge/go-1.26+-00ADD8"><img src="https://img.shields.io/badge/go-1.26+-00ADD8?logo=go" alt="Go Version"></a>
  <a href="https://snyk.io/test/github/spore-host/spore-host"><img src="https://snyk.io/test/github/spore-host/spore-host/badge.svg" alt="Known Vulnerabilities"></a>
</p>

**spore.host** is a suite of CLI tools for launching and managing AWS EC2 instances — with automatic lifecycle management so instances clean up after themselves.

- 🔍 **truffle** — Find capacity, compare spot prices, check quotas
- 🚀 **spawn** — Launch and manage instances
- ⚙️ **spored** — Lifecycle daemon (runs on each instance)
- 👁️ **lagotto** — Watch for EC2 capacity and act when it appears
- 💬 **spore-bot** — Control instances from Slack or Teams
- 🤖 **spore-host-mcp** — AI assistant integration (Claude, Cursor)

---

## Installation

**macOS / Linux (Homebrew)**

```bash
brew install spore-host/tap/truffle
brew install spore-host/tap/spawn
brew install spore-host/tap/lagotto
```

**Windows (Scoop)**

```powershell
scoop bucket add spore-host https://github.com/spore-host/scoop-bucket
scoop install truffle spawn lagotto
```

**Debian / Ubuntu (.deb)**

```bash
curl -LO https://github.com/spore-host/spore-host/releases/latest/download/truffle_linux_amd64.deb
curl -LO https://github.com/spore-host/spore-host/releases/latest/download/spawn_linux_amd64.deb
curl -LO https://github.com/spore-host/spore-host/releases/latest/download/lagotto_linux_amd64.deb
sudo dpkg -i truffle_linux_amd64.deb spawn_linux_amd64.deb lagotto_linux_amd64.deb
```

**RHEL / Fedora (.rpm)**

```bash
sudo rpm -i https://github.com/spore-host/spore-host/releases/latest/download/truffle_linux_amd64.rpm
sudo rpm -i https://github.com/spore-host/spore-host/releases/latest/download/spawn_linux_amd64.rpm
sudo rpm -i https://github.com/spore-host/spore-host/releases/latest/download/lagotto_linux_amd64.rpm
```

**Direct download**

Pre-built binaries for Linux, macOS, and Windows (amd64/arm64) on the [releases page](https://github.com/spore-host/spore-host/releases/latest).

**Build from source**

```bash
git clone https://github.com/spore-host/spore-host
cd spore-host/truffle && make build && sudo make install
cd ../spawn && make build && sudo make install
cd ../lagotto && make build && sudo make install
```

---

## Quick Start

```bash
# Find the cheapest spot instance across regions
truffle spot c6a.xlarge c7g.xlarge --sort-by-price --active-only

# Launch with a name — gets DNS at my-job.<account>.spore.host automatically
spawn launch my-job --instance-type c6a.xlarge --ttl 4h --on-complete terminate

# Connect by name (auto-starts if stopped)
spawn connect my-job

# Run a command without an interactive shell
spawn connect my-job -- 'nohup bash /tmp/run.sh > /tmp/run.log 2>&1 &'

# Check status (TTL remaining, idle state, cost)
spawn status my-job

# Extend TTL without reconnecting
spawn extend my-job 2h

# List all running instances
spawn list
```

---

## The Tools

### truffle — Find Capacity

Search instance types, compare spot prices across regions and architectures, check EC2 and SageMaker quotas.

**Works without AWS credentials** — credentials only needed for `truffle quotas` and `truffle capacity`.

```bash
# Compare spot prices across instance families
truffle spot c6i.xlarge c6a.xlarge c7g.xlarge --sort-by-price --active-only

# Find GPU capacity reservations
truffle capacity --gpu-only

# Check EC2 quotas
truffle quotas --regions us-east-1 --family P

# Check SageMaker ml.* quotas (processing/training/endpoint)
truffle quotas --service sagemaker --family g5 --regions us-west-2

# Natural language search
truffle find "nvidia h100 8gpu efa"
truffle find "graviton large"
```

Output formats: `--output table` (default), `--output json`, `--output yaml`, `--output csv`

[truffle documentation →](truffle/README.md)

---

### spawn — Launch and Manage

Launch EC2 instances with automatic lifecycle management. Instances terminate themselves — no forgotten bills.

**Requires AWS credentials.**

```bash
# Launch with full lifecycle controls
spawn launch my-job \
  --instance-type c6a.xlarge \
  --ttl 4h \
  --idle-timeout 20m \
  --on-complete terminate

# Connect, check, extend
spawn connect my-analysis              # SSH by name (auto-starts if stopped)
spawn connect my-job -- 'cmd1 && cmd2' # One-shot command (compound operators work)
spawn status my-analysis               # TTL remaining, cost, idle state
spawn extend my-analysis 2h            # Extend TTL live (no reconnect needed)
spawn list                             # All running instances
spawn stop my-job                      # Stop (billing pauses, data preserved)
spawn hibernate my-job                 # Hibernate (saves RAM to disk)
spawn start my-job                     # Restart stopped/hibernated instance

# Spot instances
spawn launch my-job --instance-type c6a.xlarge --spot --ttl 8h

# MPI cluster (EFA for HPC)
spawn launch sim --instance-type hpc6a.48xlarge \
  --count 4 --mpi --efa --job-array-name sim-cluster \
  --fsx-create --fsx-s3-bucket my-data-bucket

# Parameter sweep
spawn launch hp-search \
  --instance-type g5.xlarge \
  --param-file params.yaml --estimate-only   # dry run first
```

**Default safety net:** if you set neither `--ttl` nor `--idle-timeout`, spawn automatically applies `--idle-timeout 1h`.

[spawn documentation →](spawn/README.md)

---

### spored — Lifecycle Daemon

spored runs inside your instance as a systemd service. It watches for three termination triggers:

| Trigger | How to set | What happens |
|---------|-----------|--------------|
| **Completion signal** | `spored complete` or `touch /tmp/SPAWN_COMPLETE` | Terminates after grace period |
| **Idle timeout** | `--idle-timeout 20m` | Terminates after N minutes of inactivity |
| **TTL** | `--ttl 4h` | Hard deadline — terminates regardless |

Configuration is read from EC2 instance tags. Use `spawn extend` to push the deadline forward on a running instance without reconnecting. Use `spored status` on the instance to see TTL, cost, and idle state.

```bash
# From inside the instance
spored status                          # TTL remaining, cost, idle metrics
spored complete --status success       # signal completion and trigger on-complete
spored config set idle-timeout 2h      # update config (writes EC2 tag, reloads)
```

---

### lagotto — Watch for Capacity

Some instance types — particularly high-demand GPU families — aren't always available. Lagotto runs as a serverless Lambda that polls for capacity on a schedule and acts when it appears.

```bash
# Watch for any p5 instance and send a notification when available
lagotto watch "p5.*" --action notify --ttl 7d

# Watch and auto-launch when capacity appears
lagotto watch "g5.xlarge" --action spawn --spawn-config my-job.yaml

# Manage watches
lagotto list
lagotto status <watch-id>
lagotto extend <watch-id> --ttl 48h
lagotto cancel <watch-id>
lagotto history
```

**Actions:** `notify` (email/webhook/SNS), `spawn` (auto-launch with config file), `hold` (record only)

Lagotto deploys as a CloudFormation stack. See [lagotto/DEPLOYMENT.md](lagotto/DEPLOYMENT.md) for setup.

[lagotto documentation →](docs/tools/lagotto.md)

---

### spore-bot — Chat Control

Control instances from Slack or Microsoft Teams without opening a terminal.

```
/spore list                    — all your registered instances
/spore status rstudio          — current state, URL, TTL countdown
/spore extend rstudio 4h       — extend TTL from chat
/spore stop rstudio            — stop without SSH
```

Register instances and users with `spawn notify register`. See the [Slack Setup guide](docs/guides/slack-setup.md) or [Teams Setup guide](docs/guides/teams-setup.md).

---

### spore-host-mcp — AI Assistant Integration

The spore.host MCP server exposes truffle and spawn as tools for AI assistants that support the Model Context Protocol (Claude Desktop, Cursor, and others).

```bash
brew install spore-host/tap/spore-host-mcp
```

> *"Find me the cheapest A100 instance with EFA in us-east-1"*
> *"What instances do I have running and how long until they terminate?"*

See the [MCP Setup guide](docs/guides/mcp-setup.md).

---

## Examples

### Find cheapest capacity, then launch

```bash
# Check spot prices
truffle spot c6i.xlarge c6a.xlarge c7g.xlarge --sort-by-price

# Launch spot with auto-terminate on completion
spawn launch my-analysis \
  --instance-type c6a.xlarge \
  --region us-east-2 \
  --spot \
  --ttl 4h \
  --on-complete terminate
```

### Job with completion signal

```bash
# job.sh — runs on the instance
#!/bin/bash
python analyze.py --input data.csv --output results/
spored complete --status success --message "analysis done"
```

```bash
spawn launch my-analysis \
  --instance-type t4g.medium \
  --ttl 4h \
  --on-complete terminate
spawn connect my-analysis -- 'bash /tmp/job.sh &'
```

### Parameter sweep

```bash
spawn launch hp-search \
  --instance-type g5.xlarge \
  --param-file params.yaml \
  --max-concurrent 4 \
  --on-complete terminate

spawn sweep list                         # progress
spawn sweep collect <sweep-id>           # download results
```

### MPI cluster with shared FSx storage

```bash
spawn launch fem-sim \
  --instance-type hpc6a.48xlarge \
  --count 4 --mpi --efa \
  --job-array-name fem-cluster \
  --fsx-create --fsx-s3-bucket my-data \
  --ttl 8h --on-complete stop
```

### GPU instance with quota check

```bash
truffle quotas --regions us-east-1 --family P   # Check P-instance quota
truffle capacity --gpu-only                      # Find available reservations
spawn launch gpu-job --instance-type p4d.24xlarge --ttl 24h
```

### Watch for scarce GPU capacity

```bash
# Create a spawn config for when capacity appears
cat > my-training.yaml <<EOF
instance_type: p5.48xlarge
ttl: 24h
on_complete: terminate
EOF

lagotto watch "p5.48xlarge" \
  --action spawn \
  --spawn-config my-training.yaml \
  --regions us-east-1,us-west-2 \
  --ttl 7d
```

---

## Key Features

- **Auto-termination** — TTL, idle detection, and completion signal keep bills predictable
- **Named instances** — `spawn launch my-job` sets EC2 Name tag and registers DNS
- **Connect by name** — `spawn connect my-job` instead of hunting instance IDs; auto-starts stopped instances
- **One-shot SSH** — `spawn connect my-job -- 'cmd && cmd2'` — compound operators, background jobs work
- **Live TTL extension** — `spawn extend my-job 2h` updates the running instance without SSH
- **Capacity watching** — lagotto polls for availability and notifies or auto-launches
- **Spot-aware** — spored handles spot interruption notices and pre-stop hooks
- **HPC ready** — EFA, MPI cluster setup, FSx Lustre integration, placement groups
- **Multi-arch** — Intel, AMD, and Graviton (ARM) all work out of the box
- **Cross-platform** — binaries for Linux, macOS, and Windows
- **Chat control** — Slack and Teams integration via spore-bot
- **AI assistant** — MCP server for Claude, Cursor, and other AI tools
- **Multilingual** — `--lang es/fr/de/ja/pt` for non-English output
- **SageMaker quotas** — `truffle quotas --service sagemaker` surfaces ml.* limits

---

## AWS Credentials

spawn, lagotto, and `truffle quotas` require AWS credentials. truffle works without them for `search`, `spot`, `find`, and `capacity`.

```bash
# Standard AWS credential setup
aws configure

# Or environment variables
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_DEFAULT_REGION=us-east-1
```

---

## Documentation

Full documentation at **[spore.host/docs](https://spore.host)** or in the `docs/` directory.

- [Quick Start](docs/quickstart.md)
- [truffle reference](docs/tools/truffle.md)
- [spawn reference](docs/tools/spawn.md)
- [spored reference](docs/tools/spored.md)
- [lagotto reference](docs/tools/lagotto.md)
- [Slack Setup](docs/guides/slack-setup.md)
- [Teams Setup](docs/guides/teams-setup.md)
- [MCP Setup](docs/guides/mcp-setup.md)
- [MPI Clusters](docs/guides/mpi.md)
- [Parameter Sweeps](docs/guides/parameter-sweeps.md)
- [Spot Instances](docs/guides/spot-instances.md)
- [Self-Hosting](docs/guides/self-hosting.md)
- [IAM Permissions](docs/reference/iam-permissions.md)

---

## Project Structure

```
spore-host/
├── truffle/      # Instance discovery, spot prices, quotas (truffle CLI)
│   ├── cmd/      # CLI commands (search, spot, capacity, quotas, find, az, app)
│   └── pkg/      # Core packages (aws, find, metadata, output, quotas)
├── spawn/        # Instance lifecycle management (spawn CLI + spored daemon)
│   ├── cmd/      # CLI commands (launch, connect, list, status, extend, ...)
│   ├── cmd/spored/  # spored daemon binary
│   └── pkg/      # Core packages (agent, aws, dns, provider, cost, userdata, ...)
├── lagotto/      # Capacity watcher (lagotto CLI + Lambda poller)
│   ├── cmd/      # CLI commands (watch, list, status, cancel, extend, history, poll)
│   └── pkg/      # Core packages (watcher: Store, Poller, Notifier)
├── sdk/          # Language bindings
│   └── python/   # Python SDK (REST API client)
└── docs/         # VitePress documentation site
```

---

## License

Apache 2.0 — Copyright 2025-2026 Scott Friedman. See [LICENSE](LICENSE).
