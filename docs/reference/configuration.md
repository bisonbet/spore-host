# Configuration

spore.host reads configuration from three sources, in order of precedence (highest first):

1. **CLI flags** — `--ttl 8h`, `--slack-workspace T03NE3GTY`
2. **Defaults file** — `~/.spawn/config.yaml`
3. **EC2 tags** — set on the instance at launch, read by spored at startup

## Defaults file (`~/.spawn/config.yaml`)

Manage with `spawn defaults`:

```sh
spawn defaults set <key> <value>
spawn defaults unset <key>
spawn defaults list
```

### Available keys

| Key | CLI flag | Description |
|-----|----------|-------------|
| `slack-workspace` | `--slack-workspace` | Slack workspace ID for lifecycle notifications |
| `active-processes` | `--active-processes` | Process names that keep an instance alive (comma-separated) |
| `active-ports` | `--active-ports` | TCP ports that indicate activity (comma-separated) |
| `idle-timeout` | `--idle-timeout` | Default idle timeout duration |
| `hibernate-on-idle` | `--hibernate-on-idle` | Hibernate instead of terminating on idle (`true`/`false`) |

### File format

The file is YAML. You can edit it directly at `~/.spawn/config.yaml`:

```yaml
dns:
  enabled: true
  domain: spore.host

defaults:
  slack_workspace: T03NE3GTY
  idle_timeout: 1h
  active_processes: rsession
  hibernate_on_idle: false
```

## EC2 tags

Spored reads its configuration from EC2 tags on the instance at startup. These are set automatically when you use the corresponding CLI flags. You can also set them manually via the AWS console or CLI.

| Tag | Set by flag | Description |
|-----|-------------|-------------|
| `spawn:ttl` | `--ttl` | Time-to-live duration (e.g. `8h`, `2d`) |
| `spawn:idle-timeout` | `--idle-timeout` | Idle timeout duration |
| `spawn:hibernate-on-idle` | `--hibernate-on-idle` | `true` to hibernate instead of terminate |
| `spawn:active-processes` | `--active-processes` | Comma-separated process names |
| `spawn:active-ports` | `--active-ports` | Comma-separated TCP port numbers |
| `spawn:on-complete` | `--on-complete` | Action on completion: `terminate`, `stop`, `hibernate` |
| `spawn:completion-file` | `--completion-file` | Path to watch for completion signal |
| `spawn:completion-delay` | `--completion-delay` | Grace period before acting on completion |
| `spawn:pre-stop` | `--pre-stop` | Shell command to run before shutdown |
| `spawn:pre-stop-timeout` | `--pre-stop-timeout` | Max wait time for pre-stop (default: `5m`) |
| `spawn:dns-name` | `--dns` | DNS name for this instance |
| `spawn:slack-workspace-id` | `--slack-workspace` | Slack workspace ID |
| `spawn:notify-url` | (automatic) | Lambda URL for lifecycle notifications |
| `spawn:notify-command` | (automatic) | Slash command for workspace routing |
| `spawn:idle-cpu` | `--idle-cpu` | CPU percentage below which instance is considered idle |
| `spawn:managed` | (automatic) | Set to `true` on all spawn-managed instances |

::: tip
You can update tags on a running instance and spored will pick up the change on the next check cycle (every 60 seconds). This is how `spawn extend` works — it updates the `spawn:ttl` tag.
:::
