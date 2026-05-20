# Spawn

Spawn launches EC2 instances and manages their full lifecycle. It provisions the spored daemon on each instance, which handles auto-termination, idle detection, DNS, and lifecycle notifications independently of your laptop.

## Install

```sh
brew install spore-host/tap/spawn
```

## Core commands

### `spawn` / `spawn launch`

Launch an instance. With no arguments, the interactive wizard runs:

```sh
spawn
```

With flags:

```sh
spawn launch \
  --name my-instance \
  --instance-type g5.xlarge \
  --region us-east-1 \
  --ttl 8h
```

### `spawn list`

List your running (or all) instances:

```sh
spawn list
spawn list --state all
spawn list --region us-east-1
```

### `spawn status`

Detailed status for one instance:

```sh
spawn status my-instance
spawn status i-0a1b2c3d4e5f
```

### `spawn stop` / `spawn hibernate` / `spawn start`

```sh
spawn stop my-instance              # stop (billing pauses, data preserved)
spawn hibernate my-instance         # hibernate to disk (saves RAM state)
spawn start my-instance             # start stopped or hibernated instance
```

### `spawn extend`

Update the TTL on a running instance:

```sh
spawn extend my-instance 4h     # extend by 4 hours from now
```

### `spawn connect`

Open an interactive SSH session, or run a command and return. The command is wrapped in `bash -c` on the remote side, so compound operators and background jobs (`&`) work correctly:

```sh
spawn connect my-instance                                                        # interactive
spawn connect my-instance -- 'tail -20 /tmp/run.log'                            # one-shot
spawn connect my-instance -- 'cmd1 && cmd2'                                     # compound
spawn connect my-instance -- 'nohup bash /tmp/run.sh > /tmp/run.log 2>&1 &'    # background
spawn connect my-instance -- 'aws s3 cp s3://bucket/run.sh /tmp/ && bash /tmp/run.sh &'
```

When multiple instances share a name, `spawn connect` prefers the running one. Stopped or hibernated instances are automatically started before connecting — use `--no-start` to prevent this.

### `spawn defaults`

Manage default launch settings:

```sh
spawn defaults set slack-workspace T03NE3GTY
spawn defaults set idle-timeout 1h
spawn defaults set active-processes rsession
spawn defaults list
spawn defaults unset active-processes
```

Defaults are stored in `~/.spawn/config.yaml` and apply to every launch unless overridden. See [Configuration](/reference/configuration).

### `spawn notify`

Register instances and users for Slack/Teams control:

```sh
spawn notify workspace-add ...
spawn notify register ...
spawn notify enable ...
spawn notify list ...
```

See [Slack Setup](/guides/slack-setup) or [Teams Setup](/guides/teams-setup) for the full walkthrough.

## Key concepts

**TTL** — every instance has an absolute termination deadline: `launch_time + TTL`. When it fires, the instance terminates. The deadline is stored in a tag at launch and is **never reset** by stop/wake cycles — it keeps counting even while the instance is stopped. `spawn extend` pushes the deadline forward, not from now.

**Idle timeout** — spored monitors CPU, network, disk, GPU, sessions, and configured process names. When all signals indicate inactivity for the configured duration, the instance **stops** (or hibernates with `--hibernate-on-idle`). The idle timer **resets** every time the instance wakes. Idle timeout never terminates — only TTL does that.

**Spored** — a small daemon that runs on the instance, enforces the TTL deadline, detects idleness, registers DNS, and sends lifecycle notifications. Installed automatically at launch.

**Pre-stop hooks** — a shell command that runs before any lifecycle-triggered stop or termination. Use it to save checkpoints, sync output to S3, or notify downstream systems.

::: tip
See [TTL vs idle timeout](/reference/configuration#ttl-vs-idle-timeout-how-they-interact) for a complete explanation with a worked timeline.
:::

## Full command reference

→ [spawn command reference](/tools/reference/spawn)
