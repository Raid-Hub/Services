package messages

import "time"

type ParticipantStatus string

const (
	ParticipantReady             ParticipantStatus = "ready"
	ParticipantPrivatePlayerOnly ParticipantStatus = "private_player_only"
	ParticipantNoClan            ParticipantStatus = "no_clan"
	ParticipantPlayerUnresolved  ParticipantStatus = "player_unresolved"
	ParticipantClanUnresolved    ParticipantStatus = "clan_unresolved"
	ParticipantPolicyFiltered    ParticipantStatus = "policy_filtered"
)

type DeliveryChannelType string

const (
	DeliveryChannelDiscordWebhook DeliveryChannelType = "discord_webhook"
)

type ParticipantResult struct {
	MembershipId     int64             `json:"membershipId,string"`
	Status           ParticipantStatus `json:"status"`
	MembershipType   *int              `json:"membershipType,omitempty"`
	GroupId          *int64            `json:"groupId,string,omitempty"`
	IsPrivate        bool              `json:"isPrivate"`
	IdentityRepaired bool              `json:"identityRepaired"`
	ClanFromCache    bool              `json:"clanFromCache"`
	ClanResolvedAt   *time.Time        `json:"clanResolvedAt,omitempty"`
	FailureReason    *string           `json:"failureReason,omitempty"`
}

// DeliveryScope is which player/clan targets on this destination applied to this instance.
type DeliveryScope struct {
	PlayerMembershipIds []int64 `json:"playerMembershipIds,omitempty"`
	ClanGroupIds        []int64 `json:"clanGroupIds,omitempty"`
}

type SubscriptionParticipantMessage struct {
	MembershipId   int64 `json:"membershipId,string"`
	MembershipType *int  `json:"membershipType,omitempty"`
	Finished       bool  `json:"finished"`
}

// SubscriptionEventMessage is the stage 1 queue payload (routing.InstanceParticipantRefresh). See lib/services/subscriptions/README.md.
type SubscriptionEventMessage struct {
	InstanceId       int64                            `json:"instanceId,string"`
	ActivityHash     uint32                           `json:"activityHash"`
	PlayerCount      int                              `json:"playerCount"`
	DateCompleted    time.Time                        `json:"dateCompleted"`
	DurationSeconds  int                              `json:"durationSeconds"`
	Completed        bool                             `json:"completed"`
	ParticipantCount int                              `json:"participantCount"`
	Participants     []SubscriptionParticipantMessage `json:"participants"`
}

// SubscriptionMatchMessage is the stage 2 queue payload (routing.SubscriptionMatch). See lib/services/subscriptions/README.md.
type SubscriptionMatchMessage struct {
	InstanceId      int64               `json:"instanceId,string"`
	ActivityHash    uint32              `json:"activityHash"`
	PlayerCount     int                 `json:"playerCount"`
	DateCompleted   time.Time           `json:"dateCompleted"`
	DurationSeconds int                 `json:"durationSeconds"`
	Completed       bool                `json:"completed"`
	ParticipantData []ParticipantResult `json:"participantData"`
}

// SubscriptionDeliveryMessage is the stage 3 queue payload (routing.SubscriptionDelivery). See lib/services/subscriptions/README.md.
type SubscriptionDeliveryMessage struct {
	InstanceId           int64               `json:"instanceId,string"`
	DestinationChannelId int64               `json:"destinationChannelId,string"`
	ChannelType          DeliveryChannelType `json:"channelType"`
	// WebhookURL is set in the match stage so delivery workers only POST (no DB lookup).
	WebhookURL string        `json:"webhookUrl,omitempty"`
	DedupeKey  string        `json:"dedupeKey"`
	Scope      DeliveryScope `json:"scope"`
	// FireteamMembershipIds is everyone in the activity (filled by subscription_match from the instance).
	FireteamMembershipIds []int64 `json:"fireteamMembershipIds,omitempty"`
	// Raid context (filled by subscription_match for rich embeds)
	ActivityHash    uint32    `json:"activityHash,omitempty"`
	DateCompleted   time.Time `json:"dateCompleted,omitempty"`
	DurationSeconds int       `json:"durationSeconds,omitempty"`
	Completed       bool      `json:"completed,omitempty"`
	PlayerCount     int       `json:"playerCount,omitempty"`
	// EmbedPreload is filled in the match stage (DB + player/clan resolution). Required on subscription_delivery.
	EmbedPreload *DiscordEmbedPreload `json:"embedPreload,omitempty"`
}

// DiscordEmbedPreload is display data for the raid embed body (resolved before the delivery queue).
type DiscordEmbedPreload struct {
	ActivityName string `json:"activityName,omitempty"`
	VersionName  string `json:"versionName,omitempty"`
	PathSegment  string `json:"pathSegment,omitempty"`

	FireteamProfiles []DiscordFireteamProfile `json:"fireteamProfiles,omitempty"`
	InstanceStats    []DiscordInstanceStat    `json:"instanceStats,omitempty"`
	StatsUnavailable bool                     `json:"statsUnavailable,omitempty"`

	// Feats are raid skull modifiers from core.instance that exist in definitions.activity_feat_definition (icons from manifest).
	Feats []DiscordFeat `json:"feats,omitempty"`
}

// DiscordFeat is one selectable feat for the raid embed (Bungie icon + short label).
type DiscordFeat struct {
	Label   string `json:"label,omitempty"`
	IconURL string `json:"iconUrl,omitempty"`
}

type DiscordFireteamProfile struct {
	MembershipID int64  `json:"membershipId,string"`
	DisplayName  string `json:"displayName,omitempty"`
	IconURL      string `json:"iconUrl,omitempty"`
	ClassHash    uint32 `json:"classHash,omitempty"`
}

type DiscordInstanceStat struct {
	MembershipID      int64 `json:"membershipId,string"`
	Kills             int   `json:"kills"`
	Deaths            int   `json:"deaths"`
	Assists           int   `json:"assists"`
	TimePlayedSeconds int   `json:"timePlayedSeconds"`
}
