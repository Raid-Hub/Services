# Subscriptions pipeline

## What this is

When a **new raid instance** is stored, subscribed players/clans can receive notifications via:

- **`discord_webhook`** — Discord Incoming Webhook with a Components V2 raid message (activity name, fireteam, feats, PGCR link).
- **`http_callback`** — HTTPS POST with **`application/json`** body `dto.Instance` (same JSON shape as **`GET https://api.raidhub.io/instance/:id`**). Loaded once per match batch from `core.instance` via `LoadDTOInstanceFromPostgres`. Includes header **`X-RaidHub-Key`** from env `SUBSCRIPTION_HTTP_WEBHOOK_SECRET` for receivers to authenticate.

**Outbound relay (optional, `http_callback` only):** Implementation is **[Raid-Hub/subscription-webhook-relay](https://github.com/Raid-Hub/subscription-webhook-relay)**. In production the relay is often **`https://outbound-webhooks.raidhub.io/`** (`SUBSCRIPTION_WEBHOOK_RELAY_URL`). When that env is set, Hermes POSTs there **only** for **`http_callback`** destinations, with **`Authorization: Bearer`** (same value as **`X-RaidHub-Key`**, i.e. `SUBSCRIPTION_HTTP_WEBHOOK_SECRET`), **`X-RaidHub-Destination`** (partner URL from Postgres), same JSON body, and **`X-RaidHub-Key`**. **`discord_webhook` never uses this relay** — Hermes always POSTs straight to **`https://discord.com/api/webhooks/...`** (or legacy `discordapp.com`). If the relay env is unset, `http_callback` POSTs directly to the partner URL. Edge policy (e.g. Cloudflare) may still apply to `http_callback` URLs separately.

Rules live in **Postgres**; work is split across **three queue stages** so matching, hydration, and outbound HTTP stay separate.

## Why three stages (and not one worker)

1. **Different work, different scaling**
   Stage 1 resolves each player's clan via **Redis cache** (6 h TTL) with **Bungie `GetGroupsForMember`** fallback on miss. Stage 2 reads clan data from the message and does **Postgres rules + batching**. Stage 3 is **only outbound HTTP** (Discord or HTTPS JSON). Putting everything in one topic would couple retry policy, concurrency, and "is Bungie up?" to plain HTTP POSTs.

2. **Fan-out is natural at match**
   One instance can match **several destinations** (several rules). The **subscription_match** stage produces **N** `SubscriptionDeliveryMessage` values and publishes **N** queue messages. That "one logical event, many webhooks" belongs after rules are evaluated, not in storage.

3. **Delivery stays dumb on the happy path**
   Stage 2 puts **everything needed to POST** on the message (`WebhookURL` plus either `EmbedPreload` for Discord or `Instance` for `http_callback`); stage 3 only **HTTP POST**s (no Postgres). That keeps **outbound latency and failure modes** separate from **rule matching and DB reads** (stages 1-2 only).

4. **Retries are per destination**
   A failed POST should retry **that destination URL**, not re-run the whole match for the instance. Separate queues give **per-delivery retries** without redoing rule evaluation for other destinations.

## Why match before "hydrate," then fan-out

You **cannot** fully preload embed data until you know **which destinations matched** and each row's **scope** (e.g. which clan IDs apply to that destination's embed). So the order is: **apply rules -> N in-memory rows -> attach shared raid/embed data -> publish N messages**. Shared work is loaded **once per batch** (Discord: activity metadata, profiles, stats, feats; HTTP callback: `dto.Instance`) and attached to each matching row; **per-destination** scope (which players/clans) is already on each delivery message.

## End-to-end flow

```
Producer (e.g. instance_store on new instance)
    |  NewSubscriptionEvent  ->  queue: InstanceParticipantRefresh
    v
Stage 1  instance_participant_refresh
    |  PrepareParticipants: resolve clan per player (Redis cache -> Bungie fallback)
    |  populate GroupId, ClanFromCache, ClanResolvedAt on each ParticipantResult
    |  (optionally skip huge instances)
    |  publish  ->  queue: SubscriptionMatch
    v
Stage 2  subscription_match
    |  MatchEvent: read clans from message + privacy + rules -> N deliveries
    |  enrich raid fields -> destination URLs -> Discord embed preload and/or dto.Instance
    |  publish N times  ->  queue: SubscriptionDelivery
    v
Stage 3  subscription_delivery
    |  SendSubscriptionDelivery: HTTP POST (Discord webhook or JSON instance body)
    v
    done
```

Other producers (e.g. `tools/replay-subscription-instance`, which loads `core.instance` from Postgres) inject at **stage 1** by publishing the same first queue.

## Message types

`lib/messaging/messages/subscriptions.go`:

`SubscriptionEventMessage` -> `SubscriptionMatchMessage` -> `SubscriptionDeliveryMessage`

## Routing constants

`lib/messaging/routing/constants.go`: `InstanceParticipantRefresh`, `SubscriptionMatch`, `SubscriptionDelivery`.

## Hermes topic config

Each topic defines its own `TopicConfig` (no shared helper): `lib/messaging/queue-workers/instance_participant_refresh.go` (Bungie deps for scaling gates), `subscription_match.go`, `subscription_delivery.go` (HTTP-only path; no Bungie deps).

## Package layout (`lib/services/subscriptions`)

| File | Role |
|------|------|
| `logging.go` | Shared package logger |
| `subscription_event.go` | `NewSubscriptionEvent`, `PrepareParticipants` (resolves clans via `lib/services/clans`), large-instance threshold |
| `match_pipeline.go` | `MatchEvent`, rule application (reads clans from message), raid context on deliveries |
| `match_preload.go` | Stage 2 batch load: destination URLs, Discord embed hydration, `dto.Instance` for `http_callback` |
| `delivery_send.go` | `SendSubscriptionDelivery` (Discord or HTTPS JSON) |
| `discord_raid_embed.go` | Raid completion embed assembly and fireteam/clan markdown |
| `repository.go` | Rules, destinations, activity meta, matching |
| `postgres_stats.go` | Per-instance combat aggregates for embeds |
| `postgres_instance.go` | Replay loader from `core.instance` |
| `replay_setup.go` | CLI helpers for destinations/rules on replay |
