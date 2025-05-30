CREATE TABLE "_migrations" (
    "id" INT PRIMARY KEY,
    "name" TEXT NOT NULL,
    "applied_at" TIMESTAMP DEFAULT NOW()
);

CREATE TABLE "pgcr" (
    "instance_id" BIGINT NOT NULL PRIMARY KEY,
    "data" BYTEA NOT NULL,
    "date_crawled" TIMESTAMP DEFAULT NOW()
);