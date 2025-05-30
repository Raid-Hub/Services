CREATE TABLE "clan" (
    "group_id" BIGINT NOT NULL PRIMARY KEY,
    "name" TEXT NOT NULL,
    "motto" TEXT NOT NULL,
    "call_sign" TEXT NOT NULL,
    "clan_banner_data" JSONB NOT NULL,
    "updated_at" TIMESTAMP(3) WITHOUT TIME ZONE NOT NULL
);

CREATE TABLE "clan_members" (
    "group_id" BIGINT NOT NULL,
    "membership_id" BIGINT NOT NULL,
    PRIMARY KEY ("group_id", "membership_id"),
    CONSTRAINT "fk_clan_members_group_id" FOREIGN KEY ("group_id") REFERENCES "clan" ("group_id"),
    CONSTRAINT "fk_clan_members_membership_id" FOREIGN KEY ("membership_id") REFERENCES "player" ("membership_id")
);