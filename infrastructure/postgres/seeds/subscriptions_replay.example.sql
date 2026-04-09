-- Example: wire a Discord webhook to a player membership id for subscription replay testing.
-- Replace the webhook URL and membership_id, then run against Postgres (e.g. psql or Docker exec).
--
-- Pick membership_id from a ClickHouse instance:
--   SELECT players.membership_id FROM instance FINAL LIMIT 1 ARRAY JOIN players.membership_id AS membership_id;
--
-- After INSERT, note destination.id (often 1 on a fresh DB) — it appears in Hermes logs as channel_id on delivery.

INSERT INTO subscriptions.destination (channel_type, webhook_url)
VALUES (
    'discord_webhook',
    'https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN'
);

INSERT INTO subscriptions.rule (destination_id, scope, membership_id)
VALUES (
    1,
    'player',
    1234567890123456789
);
