package authdb

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"raidhub/lib/env"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

var (
	tursoMu       sync.Mutex
	tursoDB       *sql.DB
	tursoOpenErr  error
	tursoNextOpen time.Time // backoff after failed open
)

const tursoOpenRetryBackoff = 30 * time.Second

// DB returns the shared libSQL connection for the auth (Turso) database, or nil if not configured.
// Failed opens retry after tursoOpenRetryBackoff so a transient misconfig can recover without process restart.
func DB() (*sql.DB, error) {
	if env.DiscordLinkedRolesTursoURL == "" {
		return nil, nil
	}
	tursoMu.Lock()
	defer tursoMu.Unlock()
	if tursoDB != nil {
		return tursoDB, nil
	}
	now := time.Now()
	if tursoOpenErr != nil && now.Before(tursoNextOpen) {
		return nil, tursoOpenErr
	}
	db, err := sql.Open("libsql", env.DiscordLinkedRolesTursoURL)
	if err != nil {
		tursoOpenErr = err
		tursoNextOpen = now.Add(tursoOpenRetryBackoff)
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	tursoDB = db
	tursoOpenErr = nil
	return tursoDB, nil
}

// DiscordAccountRow is a linked Discord OAuth row joined from destiny_profile.
type DiscordAccountRow struct {
	AccessToken        string
	BungieMembershipID string
}

func lookupBungieByDestinyMembershipIDFromDB(ctx context.Context, destinyMembershipID string) (string, error) {
	db, err := DB()
	if err != nil {
		return "", err
	}
	if db == nil {
		return "", nil
	}
	const q = `SELECT bungie_membership_id FROM destiny_profile WHERE destiny_membership_id = ? LIMIT 1`
	var bungie sql.NullString
	if err := db.QueryRowContext(ctx, q, destinyMembershipID).Scan(&bungie); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	if !bungie.Valid || bungie.String == "" {
		return "", nil
	}
	return bungie.String, nil
}

// ListDestinyMembershipIDsByBungie returns all linked Destiny membership ids (decimal strings) for a Bungie user.
func ListDestinyMembershipIDsByBungie(ctx context.Context, bungieMembershipID string) ([]string, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	const q = `
SELECT destiny_membership_id FROM destiny_profile
WHERE bungie_membership_id = ?
ORDER BY is_primary DESC, destiny_membership_id`
	rows, err := db.QueryContext(ctx, q, bungieMembershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var did string
		if err := rows.Scan(&did); err != nil {
			return nil, err
		}
		if did != "" {
			out = append(out, did)
		}
	}
	return out, rows.Err()
}

func lookupDiscordByBungieMembershipIDFromDB(ctx context.Context, bungieMembershipID string) (*DiscordAccountRow, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	const q = `
SELECT access_token, bungie_membership_id
FROM account
WHERE provider = 'discord' AND bungie_membership_id = ?
LIMIT 1`
	row := db.QueryRowContext(ctx, q, bungieMembershipID)
	var acc DiscordAccountRow
	if err := row.Scan(&acc.AccessToken, &acc.BungieMembershipID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if acc.AccessToken == "" {
		return nil, nil
	}
	return &acc, nil
}

// LookupDiscordByDestinyMembershipID returns the Discord OAuth row for a Destiny profile, if linked (via Turso joins).
func LookupDiscordByDestinyMembershipID(ctx context.Context, destinyMembershipID string) (*DiscordAccountRow, error) {
	bungie, err := LookupBungieByDestinyMembershipID(ctx, destinyMembershipID)
	if err != nil || bungie == "" {
		return nil, err
	}
	return LookupDiscordByBungieMembershipID(ctx, bungie)
}

// LookupDiscordScopeByBungie returns the Discord OAuth scope string for a Bungie user, or nil if not linked.
func LookupDiscordScopeByBungie(ctx context.Context, bungieMembershipID string) (*string, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	const q = `SELECT scope FROM account WHERE provider = 'discord' AND bungie_membership_id = ? LIMIT 1`
	var scope sql.NullString
	if err := db.QueryRowContext(ctx, q, bungieMembershipID).Scan(&scope); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !scope.Valid {
		return nil, nil
	}
	s := scope.String
	return &s, nil
}

// LookupPrimaryDestinyMembershipID returns the primary Destiny membership id for a Bungie user, or "" if none.
func LookupPrimaryDestinyMembershipID(ctx context.Context, bungieMembershipID string) (string, error) {
	db, err := DB()
	if err != nil {
		return "", err
	}
	if db == nil {
		return "", nil
	}
	const q = `
SELECT destiny_membership_id FROM destiny_profile
WHERE bungie_membership_id = ? AND is_primary = 1
LIMIT 1`
	var did string
	if err := db.QueryRowContext(ctx, q, bungieMembershipID).Scan(&did); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return did, nil
}

func UpdateSyncOutcome(ctx context.Context, bungieMembershipID string, syncedAt time.Time, errMsg *string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	if db == nil {
		return nil
	}
	var errVal any
	if errMsg != nil {
		errVal = *errMsg
	} else {
		errVal = nil
	}
	const q = `
UPDATE account
SET discord_role_metadata_synced_at = ?,
    discord_role_metadata_sync_error = ?
WHERE provider = 'discord' AND bungie_membership_id = ?`
	_, err = db.ExecContext(ctx, q, syncedAt.Format(time.RFC3339Nano), errVal, bungieMembershipID)
	return err
}
