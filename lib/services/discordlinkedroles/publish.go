package discordlinkedroles

import (
	"context"
	"fmt"

	"raidhub/lib/database/authdb"
	"raidhub/lib/dto"
	"raidhub/lib/env"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/monitoring/global_metrics"
	"raidhub/lib/utils/logging"
)

var publishLogger = logging.NewLogger("DISCORD_LINKED_ROLES_PUBLISH")

// PublishAfterNewInstance enqueues one message per distinct Bungie user in the instance, each listing all Destiny profiles for that user (Turso).
func PublishAfterNewInstance(ctx context.Context, inst *dto.Instance) {
	if !env.DiscordLinkedRolesEnabled {
		return
	}
	if env.DiscordApplicationID == "" || env.DiscordLinkedRolesTursoURL == "" {
		publishLogger.Debug("DISCORD_ROLE_METADATA_PUBLISH_SKIPPED", map[string]any{
			"reason":            "missing_env",
			logging.INSTANCE_ID: inst.InstanceId,
		})
		return
	}
	seenBungie := map[string]struct{}{}
	for _, p := range inst.Players {
		did := fmt.Sprintf("%d", p.Player.MembershipId)
		bungie, err := authdb.LookupBungieByDestinyMembershipID(ctx, did)
		if err != nil {
			publishLogger.Warn("DISCORD_ROLE_METADATA_BUNGIE_LOOKUP_FAILED", err, map[string]any{
				"destiny_membership_id": did,
				logging.INSTANCE_ID:     inst.InstanceId,
			})
			continue
		}
		if bungie == "" {
			continue
		}
		if _, ok := seenBungie[bungie]; ok {
			continue
		}
		seenBungie[bungie] = struct{}{}

		ids, err := authdb.ListDestinyMembershipIDsByBungie(ctx, bungie)
		if err != nil {
			publishLogger.Warn("DISCORD_ROLE_METADATA_PROFILE_LIST_FAILED", err, map[string]any{
				"bungie_membership_id": bungie,
				logging.INSTANCE_ID:    inst.InstanceId,
			})
			continue
		}
		if len(ids) == 0 {
			continue
		}

		msg := messages.DiscordRoleMetadataSyncMessage{
			Trigger:              messages.TriggerInstanceNew,
			DestinyMembershipIds: ids,
			InstanceID:           inst.InstanceId,
		}
		if err := publishing.PublishJSONMessage(ctx, routing.DiscordRoleMetadataSync, msg); err != nil {
			global_metrics.DiscordLRPublishTotal.WithLabelValues("fail").Inc()
			publishLogger.Warn("DISCORD_ROLE_METADATA_PUBLISH_FAILED", err, map[string]any{
				"bungie_membership_id":   bungie,
				"destiny_membership_ids": ids,
				logging.INSTANCE_ID:      inst.InstanceId,
			})
		} else {
			global_metrics.DiscordLRPublishTotal.WithLabelValues("ok").Inc()
		}
	}
}

// PublishAccountLinkedRolesSync enqueues linked-role metadata sync for one Bungie user’s full Destiny profile list (account-initiated path; trigger account_linked_roles_sync).
func PublishAccountLinkedRolesSync(ctx context.Context, destinyMembershipIds []string) error {
	if !env.DiscordLinkedRolesEnabled {
		return nil
	}
	if env.DiscordApplicationID == "" || env.DiscordLinkedRolesTursoURL == "" {
		return nil
	}
	if len(destinyMembershipIds) == 0 {
		return nil
	}
	msg := messages.DiscordRoleMetadataSyncMessage{
		Trigger:              messages.TriggerAccountLinkedRolesSync,
		DestinyMembershipIds: destinyMembershipIds,
		InstanceID:           0,
	}
	err := publishing.PublishJSONMessage(ctx, routing.DiscordRoleMetadataSync, msg)
	if err != nil {
		global_metrics.DiscordLRPublishTotal.WithLabelValues("fail").Inc()
	} else {
		global_metrics.DiscordLRPublishTotal.WithLabelValues("ok").Inc()
	}
	return err
}
