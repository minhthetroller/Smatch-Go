-- Drop in reverse dependency order

DROP TABLE IF EXISTS "admin_audit_logs";
DROP TABLE IF EXISTS "business_profile_applications";

DROP INDEX IF EXISTS "idx_courts_owner_user_id";
ALTER TABLE "courts" DROP COLUMN IF EXISTS "owner_user_id";

DROP INDEX IF EXISTS "idx_users_roles";
ALTER TABLE "users" DROP COLUMN IF EXISTS "roles";
