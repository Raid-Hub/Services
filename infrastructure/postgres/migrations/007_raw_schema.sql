-- RaidHub Services - Raw Schema Migration
-- Raw data storage (PGCR data, can be stored on separate disk)

-- =============================================================================
-- SCHEMA CREATION
-- =============================================================================

-- Create raw schema
CREATE SCHEMA IF NOT EXISTS "raw";

-- =============================================================================
-- RAW TABLES
-- =============================================================================

-- PGCR storage table (raw Post Game Carnage Report data)
CREATE TABLE "raw"."pgcr" (
    "instance_id" BIGINT NOT NULL PRIMARY KEY,
    "data" BYTEA NOT NULL,
    "date_crawled" TIMESTAMP DEFAULT NOW()
);
