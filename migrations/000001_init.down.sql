-- Drop in reverse dependency order
ALTER TABLE "payments" DROP CONSTRAINT IF EXISTS "payments_match_player_fkey";
DROP TABLE IF EXISTS "match_players";
DROP TABLE IF EXISTS "matches";
DROP TABLE IF EXISTS "payments";
DROP TABLE IF EXISTS "bookings";
DROP TABLE IF EXISTS "users";
DROP TABLE IF EXISTS "holidays";
DROP TABLE IF EXISTS "pricing_rules";
DROP TABLE IF EXISTS "sub_court_closures";
DROP TABLE IF EXISTS "sub_courts";
DROP TABLE IF EXISTS "courts";
DROP TYPE IF EXISTS match_player_status;
DROP TYPE IF EXISTS match_status;
DROP TYPE IF EXISTS player_format;
DROP TYPE IF EXISTS shuttle_type;
DROP TYPE IF EXISTS skill_level;
