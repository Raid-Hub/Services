package queueworkers

import (
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/discordlinkedroles"
	"raidhub/lib/utils/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

var discordLinkedRolesLogger = logging.NewLogger("DISCORD_ROLE_METADATA_SYNC")

// DiscordRoleMetadataSyncTopic pushes Discord application role connection metadata (linked roles).
func DiscordRoleMetadataSyncTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:          routing.DiscordRoleMetadataSync,
		MinWorkers:         1,
		MaxWorkers:         12,
		DesiredWorkers:     2,
		KeepInReady:        true,
		PrefetchCount:      1,
		ScaleUpThreshold:   500,
		ScaleDownThreshold: 50,
		ScaleUpPercent:     0.2,
		ScaleDownPercent:   0.1,
		MaxRetryCount:      17,
		RetryDelay:         HermesOutboundHTTPRetryDelay,
	}, processDiscordRoleMetadataSync)
}

func processDiscordRoleMetadataSync(worker processing.WorkerInterface, message amqp.Delivery) error {
	msg, err := processing.ParseJSONUnretryable[messages.DiscordRoleMetadataSyncMessage](worker, message.Body)
	if err != nil {
		return err
	}

	fields := map[string]any{
		"destiny_membership_ids": msg.DestinyMembershipIds,
		logging.INSTANCE_ID:      msg.InstanceID,
		"trigger":                msg.Trigger,
	}
	worker.Debug("DISCORD_ROLE_METADATA_SYNC_START", fields)

	if err := discordlinkedroles.HandleMessage(worker.Context(), &msg); err != nil {
		discordLinkedRolesLogger.Warn("DISCORD_ROLE_METADATA_SYNC_FAILED", err, fields)
		return err
	}
	worker.Info("DISCORD_ROLE_METADATA_SYNC_DONE", fields)
	return nil
}
