# spored Agent

## What

spored (spawn daemon) is a lightweight Go daemon that runs on each spawned EC2 instance. It acts as the instance's self-management layer, enforcing lifecycle rules without requiring a persistent connection from the user's machine.

## Why

Without spored, you would need to:
- Manually terminate instances or rely on cron jobs
- Monitor for spot interruptions continuously
- Update DNS records manually
- Track idle instances yourself

spored handles all of this autonomously on the instance.

## Architecture

```
User Machine                  EC2 Instance
                              ┌─────────────────────────┐
spawn launch ────────────►   │  user-data: install spored │
                              │                           │
                              │  ┌─────────────────────┐ │
                              │  │      spored          │ │
                              │  │ ┌─────────────────┐ │ │
                              │  │ │ TTL enforcer    │ │ │
                              │  │ │ Idle detector   │ │ │
                              │  │ │ Spot monitor    │ │ │
                              │  │ │ DNS registrar   │ │ │
                              │  │ │ Metrics writer  │ │ │
                              │  │ │ Notifier        │─┼─┼──► spore-bot Lambda
                              │  │ └─────────────────┘ │ │        │
                              │  └─────────────────────┘ │        ▼
                              │                           │   Slack DMs / channel
                              │  systemd: spored.service  │
                              └─────────────────────────┘
```

## Responsibilities

### TTL Enforcement

- Reads TTL from DynamoDB instance record at startup
- Checks elapsed time every 30 seconds
- Logs warning 10 minutes before expiry
- Runs cleanup commands and terminates at TTL expiry

### Idle Detection

- Samples CPU, memory, network every 30 seconds
- Averages metrics over a sliding 5-minute window
- Terminates (or hibernates) when all metrics below thresholds for `idle_timeout`

### Spot Interruption Monitoring

- Polls EC2 instance metadata every 5 seconds
- On 2-minute warning: runs cleanup, saves outputs, terminates
- Prevents data loss from abrupt spot reclamation

### DNS Registration

- Gets public IP from EC2 metadata at startup
- Calls Route53 Lambda to create/update A record
- Monitors for IP changes (after hibernate/wake)
- Removes DNS record on termination

### Metrics

- Writes CPU, memory, disk, network metrics to DynamoDB every 60 seconds
- Metrics visible in `truffle metrics` and dashboard

### Heartbeat

- Writes a timestamp to DynamoDB every 30 seconds
- Enables health checking — missing heartbeat triggers replacement in autoscale groups

### Lifecycle Notifications

When an instance is launched with `--slack-workspace`, spored sends fire-and-forget notifications to the spore-bot Lambda at each lifecycle transition. Delivery is best-effort — notifications never delay or block lifecycle actions.

**Events fired:**

| Event | Trigger |
|-------|---------|
| `ttl_warning` | 10 minutes before TTL expiry |
| `ttl_expired` | TTL reached — immediately before termination |
| `idle_warning` | 10 minutes before idle timeout fires |
| `idle_stopped` | Idle timeout reached — before hibernate or terminate |
| `completion` | `/tmp/SPAWN_COMPLETE` sentinel file detected |
| `spot_interrupt` | EC2 Spot 2-minute interruption notice received |
| `pre_stop_start` | Pre-stop hook (`--pre-stop`) begins execution |

Each notification carries instance name, instance ID, region, DNS name, and event-specific detail. The spore-bot Lambda routes it to Slack via channel webhook (Pattern A), registered-user DMs (Pattern B), or self-service subscriptions (Pattern C). See [spore-bot — Lifecycle Notifications](../user-guide/spore-bot.md#lifecycle-notifications).

## systemd Service

spored runs as a systemd service on Amazon Linux 2023 and Ubuntu:

```bash
# View spored logs on instance
sudo journalctl -u spored -f

# Status
sudo systemctl status spored

# Restart (if needed)
sudo systemctl restart spored
```

## Configuration

spored reads config from `/etc/spored/config.yaml` (written by user-data at launch):

```yaml
instance_id: i-XXXXXXXXX
instance_name: my-instance
ttl: 4h
idle_timeout: 30m
idle_action: terminate         # or: hibernate
idle_cpu_threshold: 5
idle_memory_threshold: 20
idle_network_threshold: 10240
dynamo_table: spawn-instances
route53_lambda: spawn-dns-updater
region: us-east-1
# Slack lifecycle notifications (set automatically when launched with --slack-workspace)
notify_url: https://<spore-bot-lambda-url>
slack_workspace_id: T00000000
```

## Upgrade

spored is installed at launch from an S3 bucket. To get the latest version on a running instance:

```bash
sudo spored update
sudo systemctl restart spored
```

## Debugging

```bash
# From your machine
spawn logs my-instance --tail 100

# From the instance
sudo journalctl -u spored -n 100
sudo cat /var/log/spored/spored.log

# Test spored config
sudo spored validate
```

## Memory Footprint

spored uses approximately 10-20 MB of RAM and < 0.1% CPU.
