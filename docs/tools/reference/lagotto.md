# lagotto command reference

## Global flags

All lagotto commands inherit these persistent flags.

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--output` | `-o` | string | `table` | Output format: `table`, `json` |
| `--verbose` | `-v` | bool | `false` | Enable verbose output |
| `--watches-table` | | string | `lagotto-watches` | DynamoDB table name for watches |
| `--history-table` | | string | `lagotto-match-history` | DynamoDB table name for match history |
| `--lang` | | string | (system) | Language for output: `en`, `es`, `fr`, `de`, `ja`, `pt` |
| `--no-emoji` | | bool | `false` | Disable emoji in output |
| `--accessibility` | | bool | `false` | Enable accessibility mode (implies `--no-emoji`) |

The `--watches-table` and `--history-table` flags let you point at non-default DynamoDB tables — useful when running multiple lagotto deployments in the same account.

---

## lagotto watch

Create a capacity watch for an instance type pattern.

```
lagotto watch <instance-type-pattern>
```

Registers a watch in DynamoDB. The deployed Lambda polls on a schedule and takes the configured action when capacity matching the pattern is found. If the polling schedule was self-disabled (no active watches), `lagotto watch` re-enables it.

**Pattern syntax:** Wildcards supported — `p5.*` matches all p5 sizes, `g5.xlarge` is an exact match.

**Actions:**

| Action | Behavior |
|--------|----------|
| `notify` | Send a notification via the configured channels |
| `spawn` | Launch an instance using the provided spawn config file |
| `hold` | Record the match but take no automatic action |

**Examples:**
```sh
# Watch for any p5 instance and notify
lagotto watch "p5.*" --action notify --ttl 7d

# Watch for spot capacity under $10/hr
lagotto watch "p4d.24xlarge" --spot --max-price 10.00 --action notify

# Watch and auto-launch when capacity appears
lagotto watch "g5.xlarge" --action spawn --spawn-config my-job.yaml

# Watch specific regions only
lagotto watch "p5.48xlarge" --regions us-east-1,us-west-2
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--regions` | `-r` | strings | (all enabled) | Regions to watch (comma-separated; empty = all enabled regions) |
| `--spot` | | bool | `false` | Watch for Spot capacity (default: On-Demand) |
| `--max-price` | | float | `0` | Maximum acceptable price per hour in USD; `0` = any price |
| `--action` | | string | `notify` | Action on match: `notify`, `spawn`, `hold` |
| `--ttl` | | string | `24h` | How long to keep watching (e.g., `24h`, `7d`, `168h`) |
| `--notify` | | strings | | Notification channels: `email:user@example.com`, `webhook:https://...`, `sns:arn:...` |
| `--spawn-config` | | string | | Path to YAML file with spawn LaunchConfig (required when `--action spawn`) |

---

## lagotto list

List your active watches.

```
lagotto list [--all]
```

By default shows only active watches. Use `--all` to include expired and cancelled watches.

**Output columns:** Watch ID, Status, Pattern, Regions, Spot, Action, Expires

**Examples:**
```sh
lagotto list
lagotto list --all
lagotto list --output json
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--all` | bool | `false` | Show all statuses (default: active only) |

---

## lagotto status

Show details of a specific watch.

```
lagotto status <watch-id>
```

**Output includes:** Watch ID, status, pattern, regions, Spot flag, max price, action, created/expires timestamps, last poll time, match count, and last match details (instance type, region, AZ, price, action taken, launched instance ID if applicable).

**Examples:**
```sh
lagotto status w-a1b2c3d4
lagotto status w-a1b2c3d4 --output json
```

---

## lagotto cancel

Cancel an active watch.

```
lagotto cancel <watch-id>
```

Marks the watch as cancelled in DynamoDB. The Lambda will stop polling for it on the next cycle. Cancelled watches cannot be re-activated; create a new watch instead.

**Examples:**
```sh
lagotto cancel w-a1b2c3d4
```

---

## lagotto extend

Extend a watch's TTL.

```
lagotto extend <watch-id> [--ttl <duration>]
```

Sets the watch expiry to `now + TTL`. If the watch has already expired, it is also reactivated (status set back to `active`) and the polling schedule is re-enabled.

**Examples:**
```sh
lagotto extend w-a1b2c3d4
lagotto extend w-a1b2c3d4 --ttl 48h
lagotto extend w-a1b2c3d4 --ttl 7d
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--ttl` | string | `24h` | New TTL from now (e.g., `24h`, `7d`) |

---

## lagotto history

Show the match history for your watches.

```
lagotto history [--watch-id <id>]
```

Without `--watch-id`, shows all matches across all your watches. With `--watch-id`, filters to a specific watch.

**Output columns:** Watch ID, Instance Type, Region, AZ, Price, Action Taken, Matched At

**Examples:**
```sh
lagotto history
lagotto history --watch-id w-a1b2c3d4
lagotto history --output json
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--watch-id` | string | | Filter to a specific watch |

---

## lagotto poll

Run one polling cycle manually.

```
lagotto poll
```

Triggers a single poll of all active watches — the same logic the Lambda runs on its schedule. Useful for testing your watches without waiting for the next scheduled poll.

**Examples:**
```sh
lagotto poll
lagotto poll --verbose
```

---

## lagotto version

Print the version number.

```
lagotto version
```

---

## lagotto completion

Generate shell completion scripts.

```
lagotto completion <shell>
```

**Supported shells:** `bash`, `zsh`, `fish`, `powershell`

**Setup examples:**
```sh
# bash
lagotto completion bash > /etc/bash_completion.d/lagotto

# zsh
lagotto completion zsh > "${fpath[1]}/_lagotto"
```
