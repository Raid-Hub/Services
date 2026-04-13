package routing

// Queue routing constants for all async processing
const (
	// Player data processing queues
	PlayerCrawl   = "player_crawl"
	ActivityCrawl = "activity_history_crawl"
	CharacterFill = "character_fill"
	ClanCrawl     = "clan_crawl"

	// PGCR processing queues
	PGCRRetry = "pgcr_blocked_retry"
	PGCRCrawl = "pgcr_crawl"

	// Instance data processing queues
	InstanceStore      = "instance_store"
	InstanceCheatCheck = "instance_cheat_check"

	// Subscription pipeline (order: refresh -> match -> delivery). See lib/services/subscriptions/README.md.
	InstanceParticipantRefresh = "instance_participant_refresh"
	SubscriptionMatch          = "subscription_match"
	SubscriptionDelivery       = "subscription_delivery"
)
