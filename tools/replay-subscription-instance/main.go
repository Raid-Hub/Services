// replay-subscription-instance loads an instance from Postgres (core.instance) and publishes InstanceParticipantRefresh.
// Use it to replay an instance_id through the subscriptions pipeline (Hermes workers).
// Hermes requires Redis (clan cache) and Zeus/Bungie (clan resolution on cache miss).
//
// Required: -instance-id must be a real core.instance instance_id (no defaults; see docs for a dev example PGCR).
//
// Optional subscription DB changes: pass -apply-subscription-setup together with one of:
// -https-callback-url (JSON http_callback) or -destination-id
// to attach rules before replay. Discord webhook destinations are created via RaidHub-API (not this tool).
// Do not commit callback URLs to .env or the repo; pass them on the CLI only.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/subscriptions"
	"raidhub/lib/utils/logging"
)

var logger = logging.NewLogger("replay-subscription-instance")

// unsetInstanceID is the flag default before the user passes -instance-id (must be set explicitly).
const unsetInstanceID int64 = -1

var (
	instanceIDFlag = flag.Int64("instance-id", unsetInstanceID,
		"required: core.instance instance_id (positive integer)")
	dryRunFlag = flag.Bool("dry-run", false,
		"Print SubscriptionEventMessage JSON and exit without writing rules or publishing to RabbitMQ")
	applySubscriptionSetupFlag = flag.Bool("apply-subscription-setup", false,
		"explicit opt-in: with -https-callback-url or -destination-id, create/update destination and player rules before replay")
	httpsCallbackURLFlag = flag.String("https-callback-url", "",
		"HTTPS URL for http_callback JSON delivery (only with -apply-subscription-setup; mutually exclusive with -destination-id)")
	destinationIDFlag = flag.Int64("destination-id", 0,
		"subscriptions.destination id (only with -apply-subscription-setup; mutually exclusive with -https-callback-url)")
)

func main() {
	logging.ParseFlags()

	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	ctx := context.Background()

	postgres.Wait()

	if *instanceIDFlag == unsetInstanceID {
		logger.Fatal("INSTANCE_ID_REQUIRED", fmt.Errorf(
			"pass -instance-id with a real core.instance instance_id (see LOCAL_DEVELOPMENT.md for a dev PGCR example)"), nil)
		return
	}
	if *instanceIDFlag <= 0 {
		logger.Fatal("INSTANCE_ID_INVALID", fmt.Errorf(
			"-instance-id must be a positive instance_id"), nil)
		return
	}

	hasHTTPSCallback := strings.TrimSpace(*httpsCallbackURLFlag) != ""
	hasDestID := *destinationIDFlag != 0
	nDestFlags := 0
	if hasHTTPSCallback {
		nDestFlags++
	}
	if hasDestID {
		nDestFlags++
	}
	if nDestFlags > 1 {
		logger.Fatal("FLAGS_CONFLICT", fmt.Errorf(
			"use only one of -destination-id or -https-callback-url"), nil)
		return
	}
	if nDestFlags != 0 && !*applySubscriptionSetupFlag {
		logger.Fatal("SUBSCRIPTION_SETUP_OPT_IN_REQUIRED", fmt.Errorf(
			"pass -apply-subscription-setup to create/update destination and rules when using -https-callback-url or -destination-id"), nil)
		return
	}
	if *applySubscriptionSetupFlag && nDestFlags == 0 {
		logger.Fatal("SUBSCRIPTION_SETUP_INCOMPLETE", fmt.Errorf(
			"-apply-subscription-setup requires -https-callback-url or -destination-id"), nil)
		return
	}

	inst, err := subscriptions.LoadDTOInstanceFromPostgres(ctx, *instanceIDFlag)
	if err != nil {
		logger.Fatal("POSTGRES_LOAD_FAILED", err, nil)
		return
	}

	membershipIDs := make([]int64, 0, len(inst.Players))
	for _, p := range inst.Players {
		membershipIDs = append(membershipIDs, p.Player.MembershipId)
	}

	if !*dryRunFlag && *applySubscriptionSetupFlag {
		var destID int64
		var created bool
		switch {
		case hasHTTPSCallback:
			destID, created, err = subscriptions.FindOrCreateDestinationByHTTPSCallback(ctx, *httpsCallbackURLFlag)
			if err != nil {
				logger.Fatal("DESTINATION_HTTPS_CALLBACK_FAILED", err, nil)
				return
			}
			logger.Info("REPLAY_DESTINATION", map[string]any{
				"destination_id": destID,
				"created":        created,
				"channel_type":   "http_callback",
			})
		case hasDestID:
			destID = *destinationIDFlag
			ok, err := subscriptions.DestinationExists(ctx, destID)
			if err != nil {
				logger.Fatal("DESTINATION_LOOKUP_FAILED", err, nil)
				return
			}
			if !ok {
				logger.Fatal("DESTINATION_NOT_FOUND", fmt.Errorf("no active subscriptions.destination with id=%d", destID), nil)
				return
			}
		}
		n, err := subscriptions.EnsurePlayerRulesForReplay(ctx, destID, membershipIDs)
		if err != nil {
			logger.Fatal("ENSURE_RULES_FAILED", err, map[string]any{"destination_id": destID})
			return
		}
		logger.Info("REPLAY_RULES_ENSURED", map[string]any{
			"destination_id": destID,
			"rules_inserted": n,
			"participants":   len(membershipIDs),
		})
	}

	event := subscriptions.NewSubscriptionEvent(inst)
	if *dryRunFlag {
		out, err := json.MarshalIndent(event, "", "  ")
		if err != nil {
			logger.Fatal("JSON_MARSHAL_FAILED", err, nil)
			return
		}
		fmt.Println(string(out))
		if *applySubscriptionSetupFlag {
			logger.Info("DRY_RUN_NO_PUBLISH", map[string]any{
				logging.INSTANCE_ID: inst.InstanceId,
				"note":              "skipped -apply-subscription-setup DB changes and RabbitMQ publish",
			})
		} else {
			logger.Info("DRY_RUN_NO_PUBLISH", map[string]any{logging.INSTANCE_ID: inst.InstanceId})
		}
		return
	}

	publishing.Wait()
	if err := publishing.PublishJSONMessage(ctx, routing.InstanceParticipantRefresh, event); err != nil {
		logger.Fatal("PUBLISH_FAILED", err, map[string]any{logging.INSTANCE_ID: inst.InstanceId})
		return
	}

	logger.Info("REPLAY_PUBLISHED_INSTANCE_PARTICIPANT_REFRESH", map[string]any{
		logging.INSTANCE_ID: inst.InstanceId,
		logging.COUNT:       len(inst.Players),
	})
}
