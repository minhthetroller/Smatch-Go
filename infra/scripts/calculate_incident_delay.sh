#!/usr/bin/env bash
# Calculates a recommended SQS delay for incident alarm log queries.
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  infra/scripts/calculate_incident_delay.sh \
    --asg-name smatch-asg \
    --alarm-name smatch-prod-backend-cpu-high \
    --log-group /smatch/prod/backend \
    [--region ap-southeast-1] [--profile default] \
    [--start-time 2026-06-08T00:00:00Z] [--end-time 2026-06-08T02:00:00Z]

The script recommends:
  min(900, round_up_to_minute(max(scale_ready_seconds, log_ingestion_lag_seconds) + 60))

If the relevant metrics or logs are not available, it prints the default 600s.
USAGE
}

REGION=""
PROFILE=""
ASG_NAME=""
ALARM_NAME=""
LOG_GROUP=""
START_TIME=""
END_TIME=""
BUFFER_SECONDS=60
DEFAULT_DELAY_SECONDS=600
LOG_FILTER_PATTERN='{ $.event = "http_request" }'

while [[ $# -gt 0 ]]; do
  case "$1" in
    --region)
      REGION="$2"
      shift 2
      ;;
    --profile)
      PROFILE="$2"
      shift 2
      ;;
    --asg-name)
      ASG_NAME="$2"
      shift 2
      ;;
    --alarm-name)
      ALARM_NAME="$2"
      shift 2
      ;;
    --log-group)
      LOG_GROUP="$2"
      shift 2
      ;;
    --start-time)
      START_TIME="$2"
      shift 2
      ;;
    --end-time)
      END_TIME="$2"
      shift 2
      ;;
    --buffer-seconds)
      BUFFER_SECONDS="$2"
      shift 2
      ;;
    --default-delay-seconds)
      DEFAULT_DELAY_SECONDS="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$ASG_NAME" || -z "$ALARM_NAME" || -z "$LOG_GROUP" ]]; then
  echo "--asg-name, --alarm-name, and --log-group are required" >&2
  usage >&2
  exit 2
fi

if [[ -z "$START_TIME" || -z "$END_TIME" ]]; then
  read -r START_TIME END_TIME < <(python3 - <<'PY'
from datetime import datetime, timedelta, timezone

end = datetime.now(timezone.utc)
start = end - timedelta(hours=2)
print(start.isoformat().replace("+00:00", "Z"), end.isoformat().replace("+00:00", "Z"))
PY
)
fi

AWS_ARGS=(--output json)
if [[ -n "$REGION" ]]; then
  AWS_ARGS+=(--region "$REGION")
fi
if [[ -n "$PROFILE" ]]; then
  AWS_ARGS+=(--profile "$PROFILE")
fi

aws_json() {
  aws "${AWS_ARGS[@]}" "$@"
}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

aws_json cloudwatch describe-alarm-history \
  --alarm-name "$ALARM_NAME" \
  --history-item-type StateUpdate \
  --start-date "$START_TIME" \
  --end-date "$END_TIME" \
  > "$TMP_DIR/alarm_history.json"

ALARM_TIME="$(python3 - "$TMP_DIR/alarm_history.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as fh:
    items = json.load(fh).get("AlarmHistoryItems", [])

alarm_times = []
for item in items:
    data = {}
    raw = item.get("HistoryData") or "{}"
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        pass
    new_state = data.get("newState", {}).get("stateValue")
    summary = item.get("HistorySummary", "")
    if new_state == "ALARM" or "ALARM" in summary:
        alarm_times.append(item.get("Timestamp"))

alarm_times = [value for value in alarm_times if value]
print(sorted(alarm_times)[-1] if alarm_times else "")
PY
)"

if [[ -z "$ALARM_TIME" ]]; then
  echo "No ALARM transition found for $ALARM_NAME between $START_TIME and $END_TIME" >&2
  echo "recommended_delay_seconds=$DEFAULT_DELAY_SECONDS"
  exit 0
fi

aws_json cloudwatch get-metric-statistics \
  --namespace AWS/AutoScaling \
  --metric-name GroupDesiredCapacity \
  --dimensions "Name=AutoScalingGroupName,Value=$ASG_NAME" \
  --start-time "$ALARM_TIME" \
  --end-time "$END_TIME" \
  --period 60 \
  --statistics Average \
  > "$TMP_DIR/desired.json"

aws_json cloudwatch get-metric-statistics \
  --namespace AWS/AutoScaling \
  --metric-name GroupInServiceInstances \
  --dimensions "Name=AutoScalingGroupName,Value=$ASG_NAME" \
  --start-time "$ALARM_TIME" \
  --end-time "$END_TIME" \
  --period 60 \
  --statistics Average \
  > "$TMP_DIR/in_service.json"

read -r ALARM_MS END_MS < <(python3 - "$ALARM_TIME" "$END_TIME" <<'PY'
from datetime import datetime, timezone
import sys

def parse(value):
    return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(timezone.utc)

alarm = parse(sys.argv[1])
end = parse(sys.argv[2])
print(int(alarm.timestamp() * 1000), int(end.timestamp() * 1000))
PY
)

aws_json logs filter-log-events \
  --log-group-name "$LOG_GROUP" \
  --start-time "$ALARM_MS" \
  --end-time "$END_MS" \
  --filter-pattern "$LOG_FILTER_PATTERN" \
  --limit 1000 \
  > "$TMP_DIR/logs.json"

python3 - "$ALARM_TIME" "$BUFFER_SECONDS" "$DEFAULT_DELAY_SECONDS" \
  "$TMP_DIR/desired.json" "$TMP_DIR/in_service.json" "$TMP_DIR/logs.json" <<'PY'
from datetime import datetime, timezone
import json
import math
import sys

alarm_time = datetime.fromisoformat(sys.argv[1].replace("Z", "+00:00")).astimezone(timezone.utc)
alarm_ms = int(alarm_time.timestamp() * 1000)
buffer_seconds = int(sys.argv[2])
default_delay_seconds = int(sys.argv[3])
desired_path, in_service_path, logs_path = sys.argv[4:7]

def load_datapoints(path):
    with open(path, encoding="utf-8") as fh:
        points = json.load(fh).get("Datapoints", [])
    return sorted(
        (
            datetime.fromisoformat(point["Timestamp"].replace("Z", "+00:00")).astimezone(timezone.utc),
            float(point.get("Average", 0)),
        )
        for point in points
        if point.get("Timestamp")
    )

desired = load_datapoints(desired_path)
in_service = load_datapoints(in_service_path)

scale_ready_seconds = 0
if desired and in_service:
    baseline = desired[0][1]
    increased = next(((ts, value) for ts, value in desired if value > baseline), None)
    if increased:
        increased_ts, target_capacity = increased
        ready = next(
            (ts for ts, value in in_service if ts >= increased_ts and value >= target_capacity),
            None,
        )
        if ready:
            scale_ready_seconds = max(0, int((ready - alarm_time).total_seconds()))

with open(logs_path, encoding="utf-8") as fh:
    events = json.load(fh).get("events", [])

log_ingestion_lag_seconds = 0
for event in events:
    ingestion_ms = event.get("ingestionTime")
    if isinstance(ingestion_ms, int) and ingestion_ms >= alarm_ms:
        log_ingestion_lag_seconds = max(
            log_ingestion_lag_seconds,
            int((ingestion_ms - alarm_ms) / 1000),
        )

observed_seconds = max(scale_ready_seconds, log_ingestion_lag_seconds)
if observed_seconds <= 0:
    recommended = default_delay_seconds
else:
    recommended = int(math.ceil((observed_seconds + buffer_seconds) / 60) * 60)
    recommended = min(900, recommended)

print(f"alarm_time={alarm_time.isoformat().replace('+00:00', 'Z')}")
print(f"scale_ready_seconds={scale_ready_seconds}")
print(f"log_ingestion_lag_seconds={log_ingestion_lag_seconds}")
print(f"recommended_delay_seconds={recommended}")
if recommended == 900 and observed_seconds + buffer_seconds > 900:
    print("warning=SQS delay is capped at 900 seconds; consider Step Functions Wait or EventBridge Scheduler")
PY
