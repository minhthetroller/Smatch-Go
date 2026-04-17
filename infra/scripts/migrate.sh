#!/usr/bin/env bash
# infra/scripts/migrate.sh
# Runs all *.up.sql migration files against the database at DATABASE_URL.
# Requires: psql
# Usage:
#   DATABASE_URL="postgresql://..." bash infra/scripts/migrate.sh
#   (init.sh sets DATABASE_URL automatically)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MIGRATIONS_DIR="$REPO_ROOT/migrations"

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "ERROR: DATABASE_URL is not set."
  exit 1
fi

log() { echo "[migrate] $*"; }

log "Running migrations from $MIGRATIONS_DIR..."

for sql_file in $(ls "$MIGRATIONS_DIR"/*.up.sql | sort); do
  log "Applying: $(basename "$sql_file")..."
  psql "$DATABASE_URL" -f "$sql_file"
  log "Done: $(basename "$sql_file")"
done

log "All migrations applied successfully."
