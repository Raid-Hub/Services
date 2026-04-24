package subscriptions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"raidhub/lib/database/postgres"
)

// ClanNameMatch is one row from clan.clan (e.g. LookupClanByGroupID).
type ClanNameMatch struct {
	GroupID int64
	Name    string
}

// RuleInstanceCriteria maps to subscriptions.rule require_* and activity_raid_bitmap (AND semantics in the matcher).
type RuleInstanceCriteria struct {
	RequireFresh       bool
	RequireCompleted   bool
	ActivityRaidBitmap uint64 // Stored NOT NULL; 0 = all raids, non-zero = filter (OR of raid bits).
}

// EnsureClanRule inserts an active clan-scoped rule if none exists for this destination + group_id.
// Does not change require_* on an existing row; use UpsertClanRuleWithInstanceCriteria to set filters.
func EnsureClanRule(ctx context.Context, destinationID, groupID int64) (inserted bool, err error) {
	res, err := postgres.DB.ExecContext(ctx, `
		INSERT INTO subscriptions.rule (destination_id, scope, group_id)
		SELECT $1, 'clan', $2
		WHERE NOT EXISTS (
			SELECT 1 FROM subscriptions.rule r
			WHERE r.destination_id = $1
			  AND r.scope = 'clan'
			  AND r.group_id = $2
			  AND r.is_active
		)`, destinationID, groupID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UpsertClanRuleWithInstanceCriteria inserts a clan rule with instance gates, or updates require_* if an active row already exists.
func UpsertClanRuleWithInstanceCriteria(ctx context.Context, destinationID, groupID int64, cr RuleInstanceCriteria) (inserted bool, err error) {
	res, err := postgres.DB.ExecContext(ctx, `
		INSERT INTO subscriptions.rule (destination_id, scope, group_id, require_fresh, require_completed, activity_raid_bitmap)
		SELECT $1, 'clan', $2, $3, $4, $5
		WHERE NOT EXISTS (
			SELECT 1 FROM subscriptions.rule r
			WHERE r.destination_id = $1
			  AND r.scope = 'clan'
			  AND r.group_id = $2
			  AND r.is_active
		)`, destinationID, groupID, cr.RequireFresh, cr.RequireCompleted, int64(cr.ActivityRaidBitmap))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return true, nil
	}
	_, err = postgres.DB.ExecContext(ctx, `
		UPDATE subscriptions.rule
		SET require_fresh = $3, require_completed = $4, activity_raid_bitmap = $5, updated_at = NOW()
		WHERE destination_id = $1 AND scope = 'clan' AND group_id = $2 AND is_active`,
		destinationID, groupID, cr.RequireFresh, cr.RequireCompleted, int64(cr.ActivityRaidBitmap))
	return false, err
}

// UpsertPlayerRulesWithInstanceCriteria inserts player rules with instance gates, or updates require_* on existing active rows.
func UpsertPlayerRulesWithInstanceCriteria(ctx context.Context, destinationID int64, membershipIDs []int64, cr RuleInstanceCriteria) (inserted, updated int, err error) {
	for _, mid := range membershipIDs {
		res, err := postgres.DB.ExecContext(ctx, `
			INSERT INTO subscriptions.rule (destination_id, scope, membership_id, require_fresh, require_completed, activity_raid_bitmap)
			SELECT $1, 'player', $2, $3, $4, $5
			WHERE NOT EXISTS (
				SELECT 1 FROM subscriptions.rule r
				WHERE r.destination_id = $1
				  AND r.scope = 'player'
				  AND r.membership_id = $2
				  AND r.is_active
			)`, destinationID, mid, cr.RequireFresh, cr.RequireCompleted, int64(cr.ActivityRaidBitmap))
		if err != nil {
			return inserted, updated, fmt.Errorf("rule for membership_id %d: %w", mid, err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			inserted++
			continue
		}
		res2, err := postgres.DB.ExecContext(ctx, `
			UPDATE subscriptions.rule
			SET require_fresh = $3, require_completed = $4, activity_raid_bitmap = $5, updated_at = NOW()
			WHERE destination_id = $1 AND scope = 'player' AND membership_id = $2 AND is_active`,
			destinationID, mid, cr.RequireFresh, cr.RequireCompleted, int64(cr.ActivityRaidBitmap))
		if err != nil {
			return inserted, updated, err
		}
		u, _ := res2.RowsAffected()
		if u > 0 {
			updated++
		}
	}
	return inserted, updated, nil
}

// LookupClanByGroupID returns the clan row for a Bungie/RaidHub clan group_id (e.g. raidhub.io/clan/<id>).
func LookupClanByGroupID(ctx context.Context, groupID int64) (*ClanNameMatch, error) {
	var m ClanNameMatch
	err := postgres.DB.QueryRowContext(ctx, `
		SELECT group_id, name FROM clan.clan WHERE group_id = $1`, groupID).Scan(&m.GroupID, &m.Name)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// LookupClanNameOptional returns the clan display name when clan.clan has a row.
// If there is no row (clan not ingested yet), found is false and err is nil — subscription rules still use group_id alone.
func LookupClanNameOptional(ctx context.Context, groupID int64) (name string, found bool, err error) {
	var n string
	e := postgres.DB.QueryRowContext(ctx, `SELECT name FROM clan.clan WHERE group_id = $1`, groupID).Scan(&n)
	if errors.Is(e, sql.ErrNoRows) {
		return "", false, nil
	}
	if e != nil {
		return "", false, e
	}
	return n, true, nil
}

// PlayerNameForLog is core.player display fields for CLI logging (membership_id -> names).
type PlayerNameForLog struct {
	MembershipID int64
	BungieName   string
	DisplayName  sql.NullString
}

// LookupPlayerNameForLog loads bungie_name and display_name by membership_id for log output only.
func LookupPlayerNameForLog(ctx context.Context, membershipID int64) (*PlayerNameForLog, error) {
	var p PlayerNameForLog
	p.MembershipID = membershipID
	err := postgres.DB.QueryRowContext(ctx, `
		SELECT membership_id, bungie_name, display_name FROM core.player WHERE membership_id = $1`,
		membershipID,
	).Scan(&p.MembershipID, &p.BungieName, &p.DisplayName)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
