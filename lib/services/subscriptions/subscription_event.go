package subscriptions

import (
	"context"
	"time"

	"raidhub/lib/dto"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/services/clans"
	"raidhub/lib/utils/logging"
)

const LargeInstanceThreshold = 25

func NewSubscriptionEvent(inst *dto.Instance) messages.SubscriptionEventMessage {
	participants := make([]messages.SubscriptionParticipantMessage, 0, len(inst.Players))
	for _, playerActivity := range inst.Players {
		participants = append(participants, messages.SubscriptionParticipantMessage{
			MembershipId:   playerActivity.Player.MembershipId,
			MembershipType: playerActivity.Player.MembershipType,
			Finished:       playerActivity.Finished,
		})
	}

	return messages.SubscriptionEventMessage{
		InstanceId:       inst.InstanceId,
		ActivityHash:     inst.Hash,
		PlayerCount:      inst.PlayerCount,
		DateCompleted:    inst.DateCompleted,
		DurationSeconds:  inst.DurationSeconds,
		Completed:        inst.Completed,
		ParticipantCount: len(participants),
		Participants:     participants,
	}
}

func PrepareParticipants(ctx context.Context, event messages.SubscriptionEventMessage) (messages.SubscriptionMatchMessage, error) {
	if event.PlayerCount >= LargeInstanceThreshold {
		logger.Info("SUBSCRIPTIONS_SKIPPING_LARGE_INSTANCE", map[string]any{
			logging.INSTANCE_ID: event.InstanceId,
			logging.COUNT:       event.PlayerCount,
		})
		return messages.SubscriptionMatchMessage{}, nil
	}

	participantResults := make([]messages.ParticipantResult, 0, len(event.Participants))
	for _, participant := range event.Participants {
		result := messages.ParticipantResult{
			MembershipId:   participant.MembershipId,
			MembershipType: participant.MembershipType,
		}

		if participant.MembershipType == nil || *participant.MembershipType == 0 {
			reason := "membership_type_missing"
			result.Status = messages.ParticipantPlayerUnresolved
			result.FailureReason = &reason
			participantResults = append(participantResults, result)
			continue
		}

		groupId, fromCache, err := clans.ResolveClan(ctx, *participant.MembershipType, participant.MembershipId)
		if err != nil {
			return messages.SubscriptionMatchMessage{}, err
		}

		now := time.Now()
		result.ClanFromCache = fromCache
		result.ClanResolvedAt = &now

		if groupId != nil {
			result.GroupId = groupId
			result.Status = messages.ParticipantReady
		} else {
			result.Status = messages.ParticipantNoClan
		}

		participantResults = append(participantResults, result)
	}

	return messages.SubscriptionMatchMessage{
		InstanceId:      event.InstanceId,
		ActivityHash:    event.ActivityHash,
		PlayerCount:     event.PlayerCount,
		DateCompleted:   event.DateCompleted,
		DurationSeconds: event.DurationSeconds,
		Completed:       event.Completed,
		ParticipantData: participantResults,
	}, nil
}
