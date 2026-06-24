// enqueue-cheat-check publishes instance IDs to the instance_cheat_check queue for Hermes.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/utils/logging"

	"github.com/lib/pq"
)

var logger = logging.NewLogger("enqueue-cheat-check")

func main() {
	activityStr := flag.String("activity", "", "Comma-separated activity IDs (e.g. 101,102 for Pantheon)")
	versionStr := flag.String("version", "", "Comma-separated version IDs (optional)")
	instanceStr := flag.String("instance-ids", "", "Comma-separated instance IDs (optional, skips activity/version filters)")
	sinceStr := flag.String("since", "", "Only instances with date_completed >= this time (RFC3339)")
	untilStr := flag.String("until", "", "Only instances with date_completed < this time (RFC3339)")
	skipCurrent := flag.Bool("skip-current", false, "Skip instances that already have a flag_instance row at the current cheat check version")
	completedOnly := flag.Bool("completed-only", true, "Only enqueue completed instances")
	limit := flag.Int("limit", 0, "Max instances to enqueue (0 = no limit)")
	dryRun := flag.Bool("dry-run", false, "Count matching instances without publishing to RabbitMQ")
	logging.ParseFlags()

	activities, err := parseIntList(*activityStr)
	if err != nil {
		logger.Fatal("INVALID_ACTIVITY", err, nil)
	}
	versions, err := parseIntList(*versionStr)
	if err != nil {
		logger.Fatal("INVALID_VERSION", err, nil)
	}
	instanceIDs, err := parseInt64List(*instanceStr)
	if err != nil {
		logger.Fatal("INVALID_INSTANCE_IDS", err, nil)
	}

	var since, until *time.Time
	if strings.TrimSpace(*sinceStr) != "" {
		t, err := time.Parse(time.RFC3339, *sinceStr)
		if err != nil {
			logger.Fatal("INVALID_SINCE", err, map[string]any{"since": *sinceStr})
		}
		since = &t
	}
	if strings.TrimSpace(*untilStr) != "" {
		t, err := time.Parse(time.RFC3339, *untilStr)
		if err != nil {
			logger.Fatal("INVALID_UNTIL", err, map[string]any{"until": *untilStr})
		}
		until = &t
	}

	if len(instanceIDs) == 0 && len(activities) == 0 && len(versions) == 0 && since == nil && until == nil {
		logger.Fatal("MISSING_SCOPE", fmt.Errorf(
			"pass at least one scope flag: -activity, -version, -since, -until, or -instance-ids"), nil)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	postgres.Wait()
	if !*dryRun {
		publishing.Wait()
	}

	currentVersion := cheat_detection.CheatCheckVersion

	query, args := buildQuery(buildQueryParams{
		activities:     activities,
		versions:       versions,
		instanceIDs:    instanceIDs,
		since:          since,
		until:          until,
		skipCurrent:    *skipCurrent,
		completedOnly:  *completedOnly,
		currentVersion: currentVersion,
		limit:          *limit,
	})

	logger.Info("ENQUEUE_STARTED", map[string]any{
		"activities":     activities,
		"versions":       versions,
		"instance_ids":   len(instanceIDs),
		"since":          formatTime(since),
		"until":          formatTime(until),
		"skip_current":   *skipCurrent,
		"completed_only": *completedOnly,
		"limit":          *limit,
		"dry_run":        *dryRun,
		"cheat_version":  currentVersion,
	})

	rows, err := postgres.DB.QueryContext(ctx, query, args...)
	if err != nil {
		logger.Fatal("QUERY_ERROR", err, nil)
	}
	defer rows.Close()

	var (
		instanceID int64
		published  int
		failed     int
		sampleIDs  []int64
	)
	for rows.Next() {
		if err := rows.Scan(&instanceID); err != nil {
			logger.Fatal("SCAN_ERROR", err, nil)
		}
		if len(sampleIDs) < 10 {
			sampleIDs = append(sampleIDs, instanceID)
		}
		if *dryRun {
			published++
			continue
		}
		if err := publishing.PublishInt64Message(ctx, routing.InstanceCheatCheck, instanceID); err != nil {
			logger.Error("PUBLISH_ERROR", err, map[string]any{
				logging.INSTANCE_ID: instanceID,
			})
			failed++
			continue
		}
		published++
		if published%5000 == 0 {
			logger.Info("PUBLISH_PROGRESS", map[string]any{
				"published": published,
				"failed":    failed,
			})
		}
	}
	if err := rows.Err(); err != nil {
		logger.Fatal("ROWS_ERROR", err, nil)
	}

	logger.Info("ENQUEUE_COMPLETE", map[string]any{
		"dry_run":             *dryRun,
		"published":           published,
		"failed":              failed,
		"sample_instance_ids": sampleIDs,
		"cheat_version":       currentVersion,
	})
}

type buildQueryParams struct {
	activities     []int
	versions       []int
	instanceIDs    []int64
	since          *time.Time
	until          *time.Time
	skipCurrent    bool
	completedOnly  bool
	currentVersion string
	limit          int
}

func buildQuery(p buildQueryParams) (string, []any) {
	var (
		conds []string
		args  []any
		n     = 1
	)

	if len(p.instanceIDs) > 0 {
		conds = append(conds, fmt.Sprintf("i.instance_id = ANY($%d)", n))
		args = append(args, pq.Array(p.instanceIDs))
		n++
	} else {
		conds = append(conds, "TRUE")
	}

	joinActivity := len(p.activities) > 0 || len(p.versions) > 0
	joinSQL := ""
	if joinActivity {
		joinSQL = "JOIN definitions.activity_version av ON av.hash = i.hash"
	}

	if len(p.activities) > 0 {
		conds = append(conds, fmt.Sprintf("av.activity_id = ANY($%d)", n))
		args = append(args, pq.Array(p.activities))
		n++
	}
	if len(p.versions) > 0 {
		conds = append(conds, fmt.Sprintf("av.version_id = ANY($%d)", n))
		args = append(args, pq.Array(p.versions))
		n++
	}
	if p.since != nil {
		conds = append(conds, fmt.Sprintf("i.date_completed >= $%d", n))
		args = append(args, *p.since)
		n++
	}
	if p.until != nil {
		conds = append(conds, fmt.Sprintf("i.date_completed < $%d", n))
		args = append(args, *p.until)
		n++
	}
	if p.completedOnly {
		conds = append(conds, "i.completed")
	}
	if p.skipCurrent {
		conds = append(conds, fmt.Sprintf(`NOT EXISTS (
			SELECT 1
			FROM flagging.flag_instance fi
			WHERE fi.instance_id = i.instance_id
				AND fi.cheat_check_version = $%d
		)`, n))
		args = append(args, p.currentVersion)
		n++
	}

	limitSQL := ""
	if p.limit > 0 {
		limitSQL = fmt.Sprintf("LIMIT %d", p.limit)
	}

	query := fmt.Sprintf(`
		SELECT i.instance_id
		FROM core.instance i
		%s
		WHERE %s
		ORDER BY i.instance_id
		%s`,
		joinSQL,
		strings.Join(conds, "\n\t\t\tAND "),
		limitSQL,
	)
	return query, args
}

func parseIntList(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %q: %w", part, err)
		}
		out = append(out, v)
	}
	return out, nil
}

func parseInt64List(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid instance id %q: %w", part, err)
		}
		out = append(out, v)
	}
	return out, nil
}

func formatTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
