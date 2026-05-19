package messages

// Rabbit JSON trigger values for DiscordRoleMetadataSyncMessage.
const (
	TriggerInstanceNew            = "instance_new"
	TriggerAccountLinkedRolesSync = "account_linked_roles_sync"
)

// DiscordRoleMetadataSyncMessage is queued after a new raid instance is stored (see discordlinkedroles.PublishAfterNewInstance)
// or when the account path requests a linked-role metadata sync (TriggerAccountLinkedRolesSync, InstanceID 0; see API → queue).
// DestinyMembershipIds must list every Destiny profile for one Bungie user so the worker can sum clears in Postgres.
type DiscordRoleMetadataSyncMessage struct {
	Trigger              string   `json:"trigger"`
	DestinyMembershipIds []string `json:"destinyMembershipIds"`
	InstanceID           int64    `json:"instanceId"`
}
