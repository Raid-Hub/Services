// replay-subscription-instance loads an instance from ClickHouse and publishes InstanceParticipantRefresh.
// Use this as the default CLI to replay an instance_id through the subscriptions pipeline (Hermes workers).
// For dev-only Postgres seeding + two test destinations, see tools/subscription-pipeline-seed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"raidhub/lib/database/clickhouse"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/subscriptions"
	"raidhub/lib/utils/logging"
)

var logger = logging.NewLogger("replay-subscription-instance")

// defaultSubscriptionReplayInstanceID is the usual dev instance wired for subscription E2E (Vow of the Disciple, 2 players).
// Override with -instance-id, or pass 0 to use the most recent instance in ClickHouse by date_completed.
const defaultSubscriptionReplayInstanceID int64 = 16818312483

var (
	instanceIDFlag = flag.Int64("instance-id", defaultSubscriptionReplayInstanceID, "ClickHouse instance_id (0 = most recent by date_completed)")
	dryRunFlag     = flag.Bool("dry-run", false, "Print SubscriptionEventMessage JSON and exit without publishing to RabbitMQ")
)

func main() {
	logging.ParseFlags()

	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	ctx := context.Background()

	clickhouse.Wait()
	postgres.Wait()

	inst, err := subscriptions.LoadDTOInstanceFromClickHouse(ctx, *instanceIDFlag)
	if err != nil {
		logger.Fatal("CLICKHOUSE_LOAD_FAILED", err, nil)
		return
	}

	event := subscriptions.NewSubscriptionEvent(inst)
	if *dryRunFlag {
		out, err := json.MarshalIndent(event, "", "  ")
		if err != nil {
			logger.Fatal("JSON_MARSHAL_FAILED", err, nil)
			return
		}
		fmt.Println(string(out))
		logger.Info("DRY_RUN_NO_PUBLISH", map[string]any{logging.INSTANCE_ID: inst.InstanceId})
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
