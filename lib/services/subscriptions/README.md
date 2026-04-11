# Subscription Discord pipeline

## What this is

When a **new raid instance** is stored, subscribed players/clans can get a **Discord webhook** with a rich embed (activity name, fireteam stats, clan names, PGCR link). This package implements that flow: **rules in Postgres**, **outbound HTTP to Discord**, and **three queue stages** so work is split cleanly.

## Why three stages (and not one worker)

1. **Different work, different scaling**  
   Stage 1 may need **Bungie-related APIs** (participant identity / clan resolution gates in Hermes). Stage 2 does **Postgres rules + batching**.    Stage 3 is **only HTTP** to Discord. Putting everything in one topic would couple retry policy, concurrency, and “is Bungie up?” to plain webhook POSTs.

2. **Fan-out is natural at match**  
   One instance can match **several destinations** (several rules). The **subscription_match** stage produces **N** `SubscriptionDeliveryMessage` values and publishes **N** queue messages. That “one logical event, many webhooks” belongs after rules are evaluated, not in storage.

3. **Delivery stays dumb on the happy path**  
   Stage 2 puts **everything needed to POST** on the message (`WebhookURL`, `EmbedPreload`); stage 3 only **HTTP POST**s to Discord (no Postgres). That keeps **outbound webhook latency and failure modes** separate from **rule matching and DB reads** (stages 1–2 only).

4. **Retries are per destination**  
   A failed Discord POST should retry **that webhook**, not re-run the whole match for the instance. Separate queues give **per-delivery retries** without redoing rule evaluation for other destinations.

## Why match before “hydrate,” then fan-out

You **cannot** fully preload embed data until you know **which destinations matched** and each row’s **scope** (e.g. which clan IDs apply to that destination’s embed). So the order is: **apply rules → N in-memory rows → attach shared raid/embed data → publish N messages**. Shared work (activity metadata, fireteam profiles, instance stats) is loaded **once per batch** and copied onto each message; **per-destination** pieces (e.g. clan field for that scope) are filled per row.

## End-to-end flow

```
Producer (e.g. instance_store on new instance)
    |  NewSubscriptionEvent  ->  queue: InstanceParticipantRefresh
    v
Stage 1  instance_participant_refresh
    |  PrepareParticipants  ->  SubscriptionMatchMessage
    |  (optionally skip huge instances)
    |  publish  ->  queue: SubscriptionMatch
    v
Stage 2  subscription_match
    |  MatchEvent: rules + privacy + clans -> N deliveries
    |  enrich raid fields -> webhook URLs -> Discord embed preload
    |  publish N times  ->  queue: SubscriptionDelivery
    v
Stage 3  subscription_delivery
    |  SendSubscriptionDelivery: HTTP POST (webhook URL + embed preload from stage 2)
    v
    done
```

Other producers (e.g. `tools/replay-subscription-instance`) inject at **stage 1** by publishing the same first queue.

## Message types

`lib/messaging/messages/subscriptions.go`:

`SubscriptionEventMessage` → `SubscriptionMatchMessage` → `SubscriptionDeliveryMessage`

## Routing constants

`lib/messaging/routing/constants.go`: `InstanceParticipantRefresh`, `SubscriptionMatch`, `SubscriptionDelivery`.

## Hermes topic config

Each topic defines its own `TopicConfig` (no shared helper): `instance_participant_refresh.go` (Bungie deps for scaling gates), `subscription_match.go`, `subscription_delivery.go` (HTTP-only path; no Bungie deps).
