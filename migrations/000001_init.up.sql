-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ==================== COURTS ====================

CREATE TABLE IF NOT EXISTS "courts" (
    "id"               UUID         NOT NULL DEFAULT uuid_generate_v4(),
    "name"             VARCHAR(255) NOT NULL,
    "description"      TEXT,
    "phone_numbers"    TEXT[],
    "address_street"   VARCHAR(255),
    "address_ward"     VARCHAR(100),
    "address_district" VARCHAR(100),
    "address_city"     VARCHAR(100) DEFAULT 'Hà Nội',
    "details"          JSONB        NOT NULL DEFAULT '{}',
    "opening_hours"    JSONB        NOT NULL DEFAULT '{}',
    "location"         geography(Point, 4326),
    "created_at"       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    "updated_at"       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT "courts_pkey" PRIMARY KEY ("id")
);

CREATE INDEX IF NOT EXISTS "idx_courts_location"   ON "courts" USING GIST ("location");
CREATE INDEX IF NOT EXISTS "idx_courts_district"   ON "courts"("address_district");
CREATE INDEX IF NOT EXISTS "idx_courts_details"    ON "courts" USING GIN ("details");
CREATE INDEX IF NOT EXISTS "idx_courts_name_trgm"  ON "courts" USING GIN ("name" gin_trgm_ops);

-- ==================== SUB-COURTS ====================

CREATE TABLE IF NOT EXISTS "sub_courts" (
    "id"          UUID         NOT NULL DEFAULT uuid_generate_v4(),
    "court_id"    UUID         NOT NULL,
    "name"        VARCHAR(100) NOT NULL,
    "description" TEXT,
    "is_active"   BOOLEAN      NOT NULL DEFAULT true,
    "created_at"  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    "updated_at"  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT "sub_courts_pkey"         PRIMARY KEY ("id"),
    CONSTRAINT "sub_courts_court_id_fkey" FOREIGN KEY ("court_id") REFERENCES "courts"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "idx_sub_courts_court_id" ON "sub_courts"("court_id");

-- ==================== SUB-COURT CLOSURES ====================

CREATE TABLE IF NOT EXISTS "sub_court_closures" (
    "id"           UUID        NOT NULL DEFAULT uuid_generate_v4(),
    "sub_court_id" UUID        NOT NULL,
    "date"         DATE        NOT NULL,
    "start_time"   TIME,
    "end_time"     TIME,
    "reason"       VARCHAR(255),
    "created_at"   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT "sub_court_closures_pkey"            PRIMARY KEY ("id"),
    CONSTRAINT "sub_court_closures_sub_court_fkey"  FOREIGN KEY ("sub_court_id") REFERENCES "sub_courts"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "idx_sub_court_closures_lookup" ON "sub_court_closures"("sub_court_id", "date");

-- ==================== PRICING RULES ====================

CREATE TABLE IF NOT EXISTS "pricing_rules" (
    "id"            UUID         NOT NULL DEFAULT uuid_generate_v4(),
    "court_id"      UUID         NOT NULL,
    "name"          VARCHAR(100) NOT NULL,
    "day_type"      VARCHAR(20)  NOT NULL,
    "start_time"    TIME         NOT NULL,
    "end_time"      TIME         NOT NULL,
    "price_per_hour" INTEGER     NOT NULL,
    "is_active"     BOOLEAN      NOT NULL DEFAULT true,
    "created_at"    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    "updated_at"    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT "pricing_rules_pkey"         PRIMARY KEY ("id"),
    CONSTRAINT "pricing_rules_court_fkey"   FOREIGN KEY ("court_id") REFERENCES "courts"("id") ON DELETE CASCADE,
    CONSTRAINT "pricing_rules_valid_day"    CHECK (day_type IN ('weekday','weekend','holiday'))
);

CREATE INDEX IF NOT EXISTS "idx_pricing_rules_court" ON "pricing_rules"("court_id", "day_type", "is_active");

-- ==================== HOLIDAYS ====================

CREATE TABLE IF NOT EXISTS "holidays" (
    "id"         UUID        NOT NULL DEFAULT uuid_generate_v4(),
    "date"       DATE        NOT NULL,
    "name"       VARCHAR(255),
    "multiplier" FLOAT       NOT NULL DEFAULT 1.0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT "holidays_pkey"        PRIMARY KEY ("id"),
    CONSTRAINT "holidays_date_unique" UNIQUE ("date")
);

CREATE INDEX IF NOT EXISTS "idx_holidays_date" ON "holidays"("date");

-- ==================== USERS ====================

CREATE TABLE IF NOT EXISTS "users" (
    "id"               UUID         NOT NULL DEFAULT uuid_generate_v4(),
    "firebase_uid"     VARCHAR(128) NOT NULL,
    "email"            VARCHAR(255),
    "username"         VARCHAR(50),
    "provider"         VARCHAR(50)  NOT NULL,
    "is_anonymous"     BOOLEAN      NOT NULL DEFAULT false,
    "first_name"       VARCHAR(100),
    "last_name"        VARCHAR(100),
    "gender"           VARCHAR(20),
    "phone_number"     VARCHAR(20),
    "photo_url"        TEXT,
    "address_street"   VARCHAR(255),
    "address_ward"     VARCHAR(100),
    "address_district" VARCHAR(100),
    "address_city"     VARCHAR(100) DEFAULT 'Hà Nội',
    "fcm_tokens"       TEXT[]       NOT NULL DEFAULT '{}',
    "created_at"       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    "updated_at"       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT "users_pkey"            PRIMARY KEY ("id"),
    CONSTRAINT "users_firebase_unique" UNIQUE ("firebase_uid"),
    CONSTRAINT "users_email_unique"    UNIQUE ("email"),
    CONSTRAINT "users_username_unique" UNIQUE ("username")
);

CREATE INDEX IF NOT EXISTS "idx_users_firebase_uid" ON "users"("firebase_uid");
CREATE INDEX IF NOT EXISTS "idx_users_email"        ON "users"("email");
CREATE INDEX IF NOT EXISTS "idx_users_username"     ON "users"("username");

-- ==================== BOOKINGS ====================

CREATE TABLE IF NOT EXISTS "bookings" (
    "id"           UUID        NOT NULL DEFAULT uuid_generate_v4(),
    "sub_court_id" UUID        NOT NULL,
    "user_id"      UUID,
    "guest_name"   VARCHAR(255),
    "guest_phone"  VARCHAR(20),
    "guest_email"  VARCHAR(255),
    "date"         DATE        NOT NULL,
    "start_time"   TIME        NOT NULL,
    "end_time"     TIME        NOT NULL,
    "total_price"  INTEGER     NOT NULL,
    "status"       VARCHAR(20) NOT NULL DEFAULT 'pending',
    "notes"        TEXT,
    "group_id"     UUID,
    "created_at"   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at"   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT "bookings_pkey"            PRIMARY KEY ("id"),
    CONSTRAINT "bookings_sub_court_fkey"  FOREIGN KEY ("sub_court_id") REFERENCES "sub_courts"("id") ON DELETE CASCADE,
    CONSTRAINT "bookings_user_fkey"       FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE SET NULL,
    CONSTRAINT "bookings_valid_hours"     CHECK (start_time < end_time),
    CONSTRAINT "bookings_valid_status"    CHECK (status IN ('pending','confirmed','cancelled','completed','failed'))
);

CREATE INDEX IF NOT EXISTS "idx_bookings_sub_court_date" ON "bookings"("sub_court_id", "date", "status");
CREATE INDEX IF NOT EXISTS "idx_bookings_date_range"     ON "bookings"("date", "start_time", "end_time");
CREATE INDEX IF NOT EXISTS "idx_bookings_user_id"        ON "bookings"("user_id");
CREATE INDEX IF NOT EXISTS "idx_bookings_group_id"       ON "bookings"("group_id");

-- ==================== PAYMENTS ====================

CREATE TABLE IF NOT EXISTS "payments" (
    "id"              UUID        NOT NULL DEFAULT uuid_generate_v4(),
    "booking_id"      UUID,
    "match_player_id" UUID,
    "payment_type"    VARCHAR(20) NOT NULL DEFAULT 'BOOKING',
    "app_trans_id"    VARCHAR(40) NOT NULL,
    "zp_trans_id"     VARCHAR(20),
    "zp_trans_token"  VARCHAR(256),
    "amount"          INTEGER     NOT NULL,
    "status"          VARCHAR(20) NOT NULL DEFAULT 'pending',
    "order_url"       TEXT,
    "callback_data"   JSONB,
    "created_at"      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    "updated_at"      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT "payments_pkey"           PRIMARY KEY ("id"),
    CONSTRAINT "payments_app_trans_unique" UNIQUE ("app_trans_id"),
    CONSTRAINT "payments_booking_fkey"   FOREIGN KEY ("booking_id") REFERENCES "bookings"("id") ON DELETE CASCADE,
    CONSTRAINT "payments_valid_type"     CHECK (payment_type IN ('BOOKING','MATCH_JOIN')),
    CONSTRAINT "payments_valid_status"   CHECK (status IN ('pending','success','failed','expired'))
);

CREATE INDEX IF NOT EXISTS "idx_payments_booking_id"      ON "payments"("booking_id");
CREATE INDEX IF NOT EXISTS "idx_payments_match_player_id" ON "payments"("match_player_id");
CREATE INDEX IF NOT EXISTS "idx_payments_status"          ON "payments"("status");
CREATE INDEX IF NOT EXISTS "idx_payments_type"            ON "payments"("payment_type");

-- ==================== MATCHES ====================

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'skill_level') THEN
        CREATE TYPE skill_level AS ENUM (
            'TBY','Y','Y_PLUS','Y_PLUS_PLUS','TBK','TB','TB_PLUS','TB_PLUS_PLUS','K','K_PLUS','GIOI'
        );
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'shuttle_type') THEN
        CREATE TYPE shuttle_type AS ENUM (
            'TC77','BASAO','YONEX_AS30','YONEX_AS40','YONEX_AS50',
            'VICTOR_MASTER_1','VICTOR_CHAMPION_1','RSL_CLASSIC','LINDAN_40','LINDAN_50','OTHER'
        );
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'player_format') THEN
        CREATE TYPE player_format AS ENUM (
            'SINGLE_MALE','SINGLE_FEMALE','DOUBLE_MALE','DOUBLE_FEMALE','MIXED_DOUBLE','ANY'
        );
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'match_status') THEN
        CREATE TYPE match_status AS ENUM (
            'OPEN','FULL','IN_PROGRESS','COMPLETED','CANCELLED'
        );
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'match_player_status') THEN
        CREATE TYPE match_player_status AS ENUM (
            'PENDING','PENDING_PAYMENT','ACCEPTED','REJECTED','LEFT','EXPIRED'
        );
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS "matches" (
    "id"            UUID              NOT NULL DEFAULT uuid_generate_v4(),
    "court_id"      UUID              NOT NULL,
    "host_user_id"  UUID              NOT NULL,
    "title"         VARCHAR(255),
    "description"   TEXT,
    "images"        TEXT[]            NOT NULL DEFAULT '{}',
    "skill_level"   skill_level       NOT NULL,
    "shuttle_type"  shuttle_type      NOT NULL,
    "player_format" player_format     NOT NULL,
    "date"          DATE              NOT NULL,
    "start_time"    TIME              NOT NULL,
    "end_time"      TIME              NOT NULL,
    "is_private"    BOOLEAN           NOT NULL DEFAULT false,
    "price"         INTEGER           NOT NULL DEFAULT 0,
    "slots_needed"  INTEGER           NOT NULL DEFAULT 1,
    "status"        match_status      NOT NULL DEFAULT 'OPEN',
    "created_at"    TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
    "updated_at"    TIMESTAMPTZ       NOT NULL DEFAULT NOW(),
    CONSTRAINT "matches_pkey"        PRIMARY KEY ("id"),
    CONSTRAINT "matches_court_fkey"  FOREIGN KEY ("court_id") REFERENCES "courts"("id") ON DELETE CASCADE,
    CONSTRAINT "matches_host_fkey"   FOREIGN KEY ("host_user_id") REFERENCES "users"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "idx_matches_court_id"      ON "matches"("court_id");
CREATE INDEX IF NOT EXISTS "idx_matches_host_user_id"  ON "matches"("host_user_id");
CREATE INDEX IF NOT EXISTS "idx_matches_date_status"   ON "matches"("date", "status");
CREATE INDEX IF NOT EXISTS "idx_matches_skill_status"  ON "matches"("skill_level", "status");

-- ==================== MATCH PLAYERS ====================

CREATE TABLE IF NOT EXISTS "match_players" (
    "id"           UUID               NOT NULL DEFAULT uuid_generate_v4(),
    "match_id"     UUID               NOT NULL,
    "user_id"      UUID               NOT NULL,
    "status"       match_player_status NOT NULL DEFAULT 'PENDING',
    "message"      TEXT,
    "position"     INTEGER,
    "requested_at" TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
    "responded_at" TIMESTAMPTZ,
    CONSTRAINT "match_players_pkey"        PRIMARY KEY ("id"),
    CONSTRAINT "match_players_match_fkey"  FOREIGN KEY ("match_id") REFERENCES "matches"("id") ON DELETE CASCADE,
    CONSTRAINT "match_players_user_fkey"   FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE,
    CONSTRAINT "match_players_unique"      UNIQUE ("match_id", "user_id")
);

CREATE INDEX IF NOT EXISTS "idx_match_players_match_status" ON "match_players"("match_id", "status");
CREATE INDEX IF NOT EXISTS "idx_match_players_user_id"      ON "match_players"("user_id");

-- Add match_player FK to payments (after match_players table exists)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'payments_match_player_fkey') THEN
        ALTER TABLE "payments"
            ADD CONSTRAINT "payments_match_player_fkey"
            FOREIGN KEY ("match_player_id") REFERENCES "match_players"("id") ON DELETE CASCADE;
    END IF;
END $$;
