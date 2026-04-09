# Local development pitfalls (Windows and tooling)

This repo targets **Go 1.25** (`go.mod`). When bringing up Docker Compose, databases, and queue workers on a developer machine—especially **Windows**—a few issues recur. This page records what goes wrong and what we did about it.

## Go toolchain (`go 1.25` in `go.mod`)

**What happens:** The Go command may try to auto-download the matching toolchain. On some setups (notably **Windows/amd64**), download can fail with something like `toolchain not available`.

**What to do:**

- Prefer installing **Go 1.25+** locally so `go build`, `make`, and `go run` for migrations work without Docker.
- If the toolchain still cannot be installed, run Go inside the official image (see [Docker-based Go commands](#docker-based-go-commands)).

## Docker Desktop

**What happens:** Commands fail with `error during connect` / daemon not running, or odd **API version** errors when the Docker engine is unhealthy.

**What to do:** Start Docker Desktop and wait until it is fully ready. If errors persist, restart Docker Desktop.

## Environment variables

**What happens:** Services **panic at startup** with a message like `required environment variables are not set`.

**What to do:**

- Start from `example.env` → copy to `.env` (`make env` can help merge missing keys).
- Keep **RabbitMQ** credentials in `.env` aligned with `docker-compose.yml` (e.g. `RABBITMQ_USER` / `RABBITMQ_PASSWORD` must match what the broker container uses). Mismatches produce connection failures that look like wrong user (`guest` vs configured user).

## RabbitMQ delayed message exchange

**What happens:** Hermes (or messaging setup) fails with **`DELAYED_EXCHANGE_PLUGIN_NOT_ACTIVE`** or similar. The stock `rabbitmq:3.13-management` image does not ship the delayed message plugin.

**What to do:** Use the **custom RabbitMQ image** built from `infrastructure/rabbitmq/Dockerfile`, which installs and enables `rabbitmq_delayed_message_exchange`. Compose in this repo is wired to build that image.

## PostgreSQL bootstrap (`setup.sql`)

**What happens:** First-time init fails when granting privileges to a **hardcoded role** (e.g. `dev`) if `POSTGRES_USER` in `.env` is something else.

**What to do:** Init scripts should grant to the user the container actually created (e.g. `CURRENT_USER` in SQL), and `.env` should stay consistent across rebuilds. If you change `POSTGRES_USER`, you may need to **recreate the Postgres volume** so `initdb` runs again.

## Windows host bind mounts vs named volumes

**What happens:** ClickHouse (and sometimes other stateful services) hit **permission** or **`rename` / filesystem** errors under data directories mounted from the Windows host.

**What to do:** Prefer **Docker named volumes** for database data in Compose instead of bind-mounting a host path. Named volumes use Linux filesystem semantics inside the VM and avoid many Windows permission issues.

## ClickHouse nested columns and `clickhouse-go`

**What happens:** Inserts fail with errors like **`expected N arguments, got M`** on `batch.Append`, or confusing type mismatches for nested player data.

**What to do:**

- The `instance` table uses **Nested** columns; the driver may flatten these into multiple array columns. Connection settings (e.g. **`flatten_nested`**) and the **number and order of arguments** to `Append` must match the table definition. Fix the mapping in code rather than papering over with ad hoc casts.

## Shell quoting and queue testing

**What happens:** Publishing JSON test payloads with shell one-liners and `rabbitmqadmin` breaks on quoting and escaping.

**What to do:** Prefer the **RabbitMQ HTTP API** or a small script that builds JSON safely (e.g. PowerShell `ConvertTo-Json` + `Invoke-RestMethod`). This is a **testing ergonomics** issue, not production app logic.

## Subscriptions pipeline replay (ClickHouse → RabbitMQ)

**Clan vs player matching** uses Postgres only: `core.player` (privacy for player-scope rules) and `clan.clan` / `clan.clan_members` (clan-scope rules). **Redis is not used** by the subscriptions workers in this repo; no Redis service is required for local E2E.

1. Run Postgres migrations so `subscriptions.destination` and `subscriptions.rule` exist (`make migrate-postgres`).
2. Insert destinations and rules yourself, or use the dev seed helper below (optional).
3. Ensure **Hermes** is running (rebuild after subscription code changes: `docker compose build hermes && docker compose up -d hermes`).
4. **Replay an instance through the pipeline** (loads from ClickHouse, publishes `instance_participant_refresh`):

```bash
go run ./tools/replay-subscription-instance/ -dry-run
go run ./tools/replay-subscription-instance/
go run ./tools/replay-subscription-instance/ -instance-id=1234567890123456789
```

The tools default to a fixed dev instance id (`16818312483`); pass **`-instance-id=0`** to pick the most recent instance by `date_completed`.

### Dev seed + player/clan smoke test (optional)

`tools/subscription-pipeline-seed` **truncates** `subscriptions` tables, inserts a test clan (`group_id = 9000000000001`) and membership for the **first** player in the instance, creates **two** destinations (player rule on the **second** player, clan rule on that group), then publishes one replay. Pass **`-webhook-url`** each run (do not store webhook secrets in committed `.env` files).

```bash
go run ./tools/subscription-pipeline-seed/ -instance-id=16818312483 -webhook-url='https://discord.com/api/webhooks/<id>/<token>'
```

Expect Hermes logs: two `PROCESSING_SUBSCRIPTION_DELIVERY` lines for the same `instance_id` with different `channel_id` values (`1` and `2` after a fresh seed). Use `-skip-seed` to only publish if you already inserted rows manually.

Hermes only POSTs to Discord for rows in `subscriptions.destination`.

## Docker-based Go commands

If you cannot use a local Go 1.25 toolchain, you can run arbitrary `go` subcommands against the repo mounted in `golang:1.25-alpine`:

**Windows (PowerShell):**

```powershell
.\scripts\go-docker.ps1 build -o $null ./apps/hermes/
.\scripts\go-docker.ps1 test ./lib/...
```

**macOS / Linux (bash):**

```bash
chmod +x scripts/go-docker.sh   # once, if needed
./scripts/go-docker.sh build -o /dev/null ./apps/hermes/
./scripts/go-docker.sh test ./lib/...
```

These scripts are slower than a native `go` install but give a consistent toolchain without changing every `Makefile` target.
