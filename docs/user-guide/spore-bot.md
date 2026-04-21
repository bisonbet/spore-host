# spore-bot — Slack & Teams Control

spore-bot lets authorized users start, stop, hibernate, and check the status of their spore.host instances directly from Slack or Microsoft Teams — no CLI, SSH, or AWS account required.

**If you are using spore.host (the hosted platform):** the bot infrastructure is already deployed. Start at [Instance Owner Setup](#instance-owner-setup).

**If you are self-hosting spore.host:** see [Self-Hosting](#self-hosting).

---

## The three roles

| Role | Who | Setup required |
|------|-----|----------------|
| **Platform Operator** | spore.host team | Already done |
| **Instance Owner** | Researcher or team lead who owns the instances | ~20 minutes |
| **Spore User** | Collaborator who types `/spore` commands | None |

In small teams the Instance Owner and Spore User may be the same person.

---

## Instance Owner Setup

*You own spore.host instances and want to let collaborators control them from Slack.*

### 1. Create the Slack app

1. Go to **https://api.slack.com/apps** → **Create New App** → **From scratch**
2. Name it `spore-bot`, select your workspace → **Create**
3. **OAuth & Permissions** → **Bot Token Scopes** → Add:
   - `commands` — slash command support
   - `chat:write` — send messages back
   - `users:read` — look up workspace members
   - `users:read.email` — look up members by email address
4. **Install App** → **Install to Workspace** → **Allow**
5. Copy the **Bot User OAuth Token** (`xoxb-...`) from the top of the OAuth & Permissions page
6. **Basic Information** → **App Credentials** → copy the **Signing Secret**
7. Your **Workspace ID** is in the Slack URL when you're in the workspace: `app.slack.com/client/T________`

> **Enterprise / university Slack:** In some organizations, IT controls app installation. If you cannot install the app yourself, ask your IT team to approve it. You can complete all remaining steps without further IT involvement.

### 2. Configure the slash command

1. **Slash Commands** → **Create New Command**
2. Fill in:
   - **Command:** `/spore`
   - **Request URL:** `https://gmkqcugnviaafk654hfnyyzfca0vyuvt.lambda-url.us-east-1.on.aws/slack`
   - **Short description:** Control your spore.host instances
   - **Usage hint:** `[status|start|stop|hibernate|url|list|help|connect] [name]`
3. **Save** — Slack verifies the URL automatically
4. **Reinstall to Workspace** when prompted → **Allow**

### 3. Register your workspace

```bash
spawn bot workspace-add \
  --platform slack \
  --workspace-id T00000000 \
  --workspace-name "My Workspace" \
  --bot-token xoxb-... \
  --signing-secret abc123...
```

**Optional — restrict commands to specific channels:**

Find a channel's ID by opening it in Slack (it appears in the URL and in Channel Settings → About).

```bash
spawn bot workspace-add \
  --platform slack \
  --workspace-id T00000000 \
  --workspace-name "My Workspace" \
  --bot-token xoxb-... \
  --signing-secret abc123... \
  --allowed-channels C12345,C67890
```

If no channels are specified, commands are accepted from any channel.

### 4. Deploy the cross-account IAM role

This grants the bot permission to control instances in your AWS account. Run once per AWS account where your instances live:

```bash
aws cloudformation deploy \
  --stack-name spawn-bot-cross-account \
  --template-file spawn/deployment/cloudformation/bot-cross-account-role.yaml \
  --capabilities CAPABILITY_NAMED_IAM \
  --parameter-overrides \
      BotLambdaRoleArn=arn:aws:iam::966362334030:role/prism-bot-PrismBotFunctionRole-U2vZFZXgWBeM \
      TagPrefix=spawn
```

Note the `RoleArn` output — you need it in the next step:

```bash
aws cloudformation describe-stacks --stack-name spawn-bot-cross-account \
  --query 'Stacks[0].Outputs[?OutputKey==`RoleArn`].OutputValue' --output text
```

### 5. Register instances for users

You have three ways to register a user — pick whichever fits your situation:

**Option A — By email address** *(simplest when you know their email)*

```bash
spawn bot register \
  --platform slack \
  --user collaborator@university.edu \
  --workspace-id T00000000 \
  --instance i-0abc123def456 \
  --nickname rstudio \
  --role-arn arn:aws:iam::435415984226:role/SpawnBotCrossAccount \
  --allow start,stop,status,hibernate,url
```

**Option B — Via connect code** *(they generate it; no email or Slack ID needed)*

Ask your collaborator to type `/spore connect` in Slack. They receive:

```
🔑 Your connect code: SPORE-3F9A2C
Share this with your workspace admin.
Code expires in 24h and can only be used once.
```

They share the code with you, and you run:

```bash
spawn bot register \
  --platform slack \
  --connect-code SPORE-3F9A2C \
  --instance i-0abc123def456 \
  --nickname rstudio \
  --role-arn arn:aws:iam::435415984226:role/SpawnBotCrossAccount
```

**Option C — By Slack member ID** *(when you have their ID directly)*

In Slack: click their name → **View profile** → `⋯` → **Copy member ID** (looks like `U0XXXXXXX`).

```bash
spawn bot register \
  --platform slack \
  --user-id U0XXXXXXX \
  --workspace-id T00000000 \
  --instance i-0abc123def456 \
  --nickname rstudio \
  --role-arn arn:aws:iam::435415984226:role/SpawnBotCrossAccount
```

### 6. Enable access

Registrations are **disabled by default**. You must explicitly enable each one:

```bash
spawn bot enable \
  --platform slack \
  --user-id U0XXXXXXX \
  --workspace-id T00000000 \
  --nickname rstudio
```

To temporarily suspend without removing the registration:

```bash
spawn bot disable --platform slack --user-id U0XXXXXXX --workspace-id T00000000 --nickname rstudio
```

---

## Spore User Commands

*No setup required. Type these in any Slack channel where the bot is installed.*

| Command | What it does |
|---------|-------------|
| `/spore status [name]` | Instance state, IP, URL, type, uptime |
| `/spore start [name]` | Start instance — posts a status card when it's running |
| `/spore stop [name]` | Stop instance |
| `/spore hibernate [name]` | Hibernate — saves RAM to disk, pauses compute billing |
| `/spore url [name]` | Get the instance URL |
| `/spore list` | Show all your registered instances |
| `/spore connect [duration]` | Get a one-time code to share with your Instance Owner |
| `/spore help` | Show available commands |

**`[name]` accepts any of:**

```
/spore status rstudio                           ← nickname (set at registration)
/spore status 98.92.241.152                     ← IP address
/spore status i-0abc123def456                   ← AWS Instance ID
/spore status rstudio.5k0zfnmq.spore.host       ← full DNS name
```

If you only have one registered instance, the name is optional:

```
/spore stop
```

### Connect codes

If you're new to a workspace and haven't been registered yet, type `/spore connect`. You'll receive a one-time code valid for 24 hours by default (your workspace admin may have set a shorter limit). Share that code with your Instance Owner — they use it to register you without needing your Slack user ID.

You can request a shorter code lifetime: `/spore connect 4h`

---

## Managing Access

### See all registered users in a workspace
```bash
spawn bot list --platform slack --workspace-id T00000000
```

### Change what a user can do
Re-register with different `--allow` flags:
```bash
spawn bot register ... --allow status,url    # read-only access
```

### Remove a user's access
```bash
spawn bot deregister \
  --platform slack \
  --user-id U0XXXXXXX \
  --workspace-id T00000000 \
  --nickname rstudio
```

### Remove all access for an entire workspace
```bash
# Preview what will be removed
spawn bot workspace-destroy --platform slack --workspace-id T00000000

# Execute (irreversible)
spawn bot workspace-destroy --platform slack --workspace-id T00000000 --confirm
```

---

## Security Model

**Access is controlled by the registry — not by Slack.**

Any workspace member who types `/spore` commands and hasn't been registered by an Instance Owner receives "no instances registered." The command does nothing. Channel restrictions are additional defense-in-depth, not the primary control.

| Layer | What it does |
|-------|-------------|
| Registry | Only registered users can issue commands |
| Enabled flag | Registrations are off by default; explicit enable required |
| Allowed actions | Each registration specifies permitted operations |
| HMAC verification | Every request cryptographically verified as coming from Slack |
| Cross-account IAM role | Bot can only act on instances in accounts that have the role deployed |
| Audit log | Every command attempt recorded with user, instance, action, result |
| Channel restriction | Optional — limits which channels accept commands |

---

## Reference

### All `spawn bot` commands

```
spawn bot register          Register an instance for a user
spawn bot deregister        Remove a registration
spawn bot enable            Enable bot access for a registration
spawn bot disable           Suspend bot access (registration stays)
spawn bot list              List registrations for a workspace

spawn bot workspace-add     Register a Slack/Teams workspace
spawn bot workspace-remove  Remove a workspace
spawn bot workspace-list    List registered workspaces
spawn bot workspace-destroy Remove workspace and all its registrations
```

### Connect code TTL

The lifetime of `/spore connect` codes has three levels:

| Level | How | Default |
|-------|-----|---------|
| Platform | `BOT_CONNECT_CODE_TTL_HOURS` Lambda env var | 24 hours |
| Workspace | `spawn bot workspace-add --connect-ttl <hours>` | Inherits platform |
| Per-code | `/spore connect 4h` | Inherits workspace |

Workspace admins can only lower the platform default, not raise it. Users can only request shorter than the workspace maximum.
