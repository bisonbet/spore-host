# Lagotto

Lagotto watches for EC2 instance capacity and acts when it appears. It runs as a serverless Lambda function — no always-on server required. Configure a watch, deploy, and Lagotto polls on a schedule until capacity shows up.

## Install

```sh
brew install spore-host/tap/lagotto
```

## The problem it solves

Some instance types — particularly high-demand GPU families like p5.48xlarge or trn1.32xlarge — have intermittent availability. You want the instance, but there's none available right now. Without Lagotto, your options are: keep trying manually, or write your own polling loop.

Lagotto automates the waiting.

## Core commands

### `lagotto watch`

Create a watch for an instance type pattern in one or more regions:

```sh
# Watch for any p5 instance and notify when available
lagotto watch "p5.*" --ttl 7d

# Limit to specific regions
lagotto watch "p5.48xlarge" --regions us-east-1,us-west-2

# Watch and notify via email or webhook
lagotto watch "p5.48xlarge" --action notify \
  --notify email:you@example.com

# Watch and auto-launch when capacity appears
lagotto watch "g5.xlarge" --action spawn \
  --spawn-config my-job.yaml

# Watch for Spot capacity under a price ceiling
lagotto watch "p4d.24xlarge" --spot --max-price 10.00
```

### `lagotto list`

```sh
lagotto list              # active watches only
lagotto list --all        # include expired and cancelled
lagotto list --output json
```

### `lagotto status`

```sh
lagotto status <watch-id>
```

### `lagotto cancel`

```sh
lagotto cancel <watch-id>
```

### `lagotto extend`

```sh
lagotto extend <watch-id> --ttl 48h
```

### `lagotto history`

```sh
lagotto history                        # all your matches
lagotto history --watch-id <watch-id>  # one watch
```

### `lagotto poll`

Manually trigger one polling cycle (useful for testing):

```sh
lagotto poll
```

## Actions

When capacity appears, Lagotto can:

| Action | What happens |
|--------|-------------|
| `notify` | Sends a notification via `--notify` channels (email, webhook, SNS) |
| `spawn` | Launches an instance using the config file given in `--spawn-config` |
| `hold` | Records the match but takes no automatic action |

## How it works

Lagotto deploys as an AWS Lambda function with an EventBridge schedule trigger. Each tick (default 5 minutes) it calls `DescribeInstanceTypeOfferings` for each watched type and region. When the type appears in an AZ, it fires the configured action.

## Deploy

Lagotto is deployed via CloudFormation — not through the CLI. See the [deployment guide](https://github.com/spore-host/spore-host/blob/main/lagotto/DEPLOYMENT.md) for the full setup: Lambda, EventBridge schedule, DynamoDB tables, and IAM role. Once deployed, the `lagotto` CLI manages watches in that infrastructure.

## Full command reference

→ [lagotto command reference](/tools/reference/lagotto)
