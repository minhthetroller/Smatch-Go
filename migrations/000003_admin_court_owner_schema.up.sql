-- ==================== USERS ROLES ====================

ALTER TABLE "users"
    ADD COLUMN IF NOT EXISTS "roles" TEXT[] NOT NULL DEFAULT '{user}';

CREATE INDEX IF NOT EXISTS "idx_users_roles" ON "users" USING GIN ("roles");

-- ==================== COURTS OWNER ====================

ALTER TABLE "courts"
    ADD COLUMN IF NOT EXISTS "owner_user_id" UUID REFERENCES "users"("id") ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS "idx_courts_owner_user_id" ON "courts"("owner_user_id");

-- ==================== BUSINESS PROFILE APPLICATIONS ====================

CREATE TABLE IF NOT EXISTS "business_profile_applications" (
    "id"                                  UUID         NOT NULL DEFAULT uuid_generate_v4(),
    "user_id"                             UUID         NOT NULL,
    "legal_representative_name"           VARCHAR(255) NOT NULL,
    "personal_id_number"                  VARCHAR(50)  NOT NULL,
    "personal_id_front_image_url"         TEXT,
    "personal_id_back_image_url"          TEXT,
    "business_registration_cert_url"      TEXT,
    "sports_business_eligibility_cert_url" TEXT,
    "fire_safety_cert_url"                TEXT,
    "tax_id_number"                       VARCHAR(50)  NOT NULL,
    "proof_of_address_url"                TEXT,
    "bank_account_number"                 VARCHAR(100) NOT NULL,
    "bank_name"                           VARCHAR(100) NOT NULL,
    "bank_branch"                         VARCHAR(100) NOT NULL,
    "bank_account_holder_name"            VARCHAR(255) NOT NULL,
    "operational_specs"                   JSONB        NOT NULL DEFAULT '{}',
    "status"                              VARCHAR(30)  NOT NULL DEFAULT 'pending',
    "admin_notes"                         TEXT,
    "submitted_at"                        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    "reviewed_at"                         TIMESTAMPTZ,
    "reviewed_by"                         UUID,
    CONSTRAINT "business_profiles_pkey"              PRIMARY KEY ("id"),
    CONSTRAINT "business_profiles_user_unique"         UNIQUE ("user_id"),
    CONSTRAINT "business_profiles_user_fkey"           FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE CASCADE,
    CONSTRAINT "business_profiles_reviewer_fkey"       FOREIGN KEY ("reviewed_by") REFERENCES "users"("id") ON DELETE SET NULL,
    CONSTRAINT "business_profiles_valid_status"        CHECK (status IN ('pending','approved','rejected','resubmit_requested'))
);

CREATE INDEX IF NOT EXISTS "idx_business_profiles_user_id" ON "business_profile_applications"("user_id");
CREATE INDEX IF NOT EXISTS "idx_business_profiles_status"   ON "business_profile_applications"("status", "submitted_at");

-- ==================== ADMIN AUDIT LOGS ====================

CREATE TABLE IF NOT EXISTS "admin_audit_logs" (
    "id"            UUID         NOT NULL DEFAULT uuid_generate_v4(),
    "admin_user_id" UUID         NOT NULL,
    "action"        VARCHAR(100) NOT NULL,
    "target_type"   VARCHAR(50),
    "target_id"     UUID,
    "details"       JSONB        NOT NULL DEFAULT '{}',
    "created_at"    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT "admin_audit_logs_pkey"       PRIMARY KEY ("id"),
    CONSTRAINT "admin_audit_logs_admin_fkey" FOREIGN KEY ("admin_user_id") REFERENCES "users"("id") ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS "idx_admin_audit_logs_admin"   ON "admin_audit_logs"("admin_user_id", "created_at");
CREATE INDEX IF NOT EXISTS "idx_admin_audit_logs_target"  ON "admin_audit_logs"("target_type", "target_id");
