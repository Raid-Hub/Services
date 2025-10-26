package routing

// Queue routing constants for all async processing
const (
	// Player data processing queues
	PlayerCrawl     = "player_crawl"
	ActivityHistory = "activity_history"
	CharacterFill   = "character_fill"
	ClanCrawl       = "clan"

	// PGCR processing queues
	PGCRBlocked    = "pgcr_blocked"
	PGCRExists     = "pgcr_exists"
	PGCRStore      = "pgcr_store"
	PGCRCheatCheck = "pgcr_cheat_check"
)
