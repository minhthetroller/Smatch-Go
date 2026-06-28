import json
import os
import time
from datetime import datetime, timezone

import boto3


logs_client = boto3.client("logs")
sns_client = boto3.client("sns")

SNS_TOPIC_ARN = os.environ["SNS_TOPIC_ARN"]
LOG_GROUPS = json.loads(os.environ.get("LOG_GROUPS_JSON", "{}"))
ALARM_SERVICE_MAP = json.loads(os.environ.get("ALARM_SERVICE_MAP_JSON", "{}"))
LOOKBACK_MINUTES = int(os.environ.get("LOOKBACK_MINUTES", "15"))
MAX_LOG_EVENTS = int(os.environ.get("MAX_LOG_EVENTS", "25"))

ERROR_LEVELS = {"warn", "warning", "error", "fatal", "panic", "dpanic"}
PROBLEM_FILTER_PATTERNS = [
    '"\\"timeout\\": true"',
    '"\\"status_class\\": \\"5xx\\""',
    'PANIC',
    'ERROR',
    'WARN'
]


def handler(event, _context):
    if is_sqs_event(event):
        return handle_sqs_event(event)

    return process_alarm_event(event)


def is_sqs_event(event):
    records = event.get("Records")
    return isinstance(records, list) and any(
        record.get("eventSource") == "aws:sqs" for record in records
    )


def handle_sqs_event(event):
    failures = []

    for record in event.get("Records", []):
        message_id = record.get("messageId")
        try:
            process_alarm_event(decode_sqs_alarm_event(record.get("body", "")))
        except Exception as exc:  # Lambda uses this response to retry only failed messages.
            print(json.dumps({
                "level": "error",
                "msg": "failed to process incident alarm sqs message",
                "message_id": message_id,
                "error": str(exc),
            }))
            if message_id:
                failures.append({"itemIdentifier": message_id})

    return {"batchItemFailures": failures}


def decode_sqs_alarm_event(body):
    decoded = json.loads(body)
    if not isinstance(decoded, dict) or "detail" not in decoded:
        raise ValueError("SQS message body is not an EventBridge alarm event")
    return decoded


def process_alarm_event(event):
    detail = event.get("detail", {})
    alarm_name = detail.get("alarmName", "unknown-alarm")

    service = resolve_service(alarm_name)
    log_group = LOG_GROUPS.get(service)

    if not log_group:
        message = build_message(event, service, None, [])
        publish(alarm_name, service, message)
        return {"published": True, "service": service, "log_group_found": False}

    # Pass 'event' into the function here
    recent_logs = find_recent_problem_logs(log_group, event) 
    message = build_message(event, service, log_group, recent_logs)
    publish(alarm_name, service, message)
    return {
        "published": True,
        "service": service,
        "log_group": log_group,
        "log_count": len(recent_logs),
    }


def resolve_service(alarm_name):
    if alarm_name in ALARM_SERVICE_MAP:
        return ALARM_SERVICE_MAP[alarm_name]

    return infer_service(alarm_name)


def infer_service(alarm_name):
    lowered = alarm_name.lower()
    if "admin" in lowered:
        return "admin"
    return "backend"


def find_recent_problem_logs(log_group, event):
    # Default to current time just in case
    end_ms = int(time.time() * 1000)

    # Try to grab the exact time the alarm fired from the event payload
    time_str = event.get("detail", {}).get("state", {}).get("timestamp") or event.get("time")

    if time_str:
        try:
            # Parse the ISO8601 string from EventBridge
            dt = datetime.fromisoformat(time_str)
            end_ms = int(dt.timestamp() * 1000)
        except Exception as e:
            print(f"Time parsing failed, falling back to current time: {e}")

    start_ms = end_ms - (LOOKBACK_MINUTES * 60 * 1000)
    events_by_key = {}

    for pattern in PROBLEM_FILTER_PATTERNS:
        for parsed in filter_problem_logs(log_group, start_ms, end_ms, pattern):
            key = event_key(parsed)
            events_by_key[key] = parsed
            if len(events_by_key) >= MAX_LOG_EVENTS:
                break
        if len(events_by_key) >= MAX_LOG_EVENTS:
            break

    events = sorted(events_by_key.values(), key=problem_sort_key, reverse=True)
    return events[:MAX_LOG_EVENTS]


def filter_problem_logs(log_group, start_ms, end_ms, filter_pattern):
    events = []
    next_token = None

    while len(events) < MAX_LOG_EVENTS:
        request = {
            "logGroupName": log_group,
            "startTime": start_ms,
            "endTime": end_ms,
            "limit": min(100, MAX_LOG_EVENTS * 4),
        }
        if filter_pattern:
            request["filterPattern"] = filter_pattern
        if next_token:
            request["nextToken"] = next_token

        response = logs_client.filter_log_events(**request)
        for raw_event in response.get("events", []):
            parsed = parse_log_event(raw_event)
            if filter_pattern or is_problem_log(parsed):
                events.append(parsed)
                if len(events) >= MAX_LOG_EVENTS:
                    break

        next_token = response.get("nextToken")
        if not next_token:
            break

    return events


def parse_log_event(event):
    raw_message = event.get("message", "")
    message = raw_message

    # 1. Strip the outer Docker JSON wrapper (if present)
    try:
        docker_outer = json.loads(raw_message)
        if isinstance(docker_outer, dict) and "log" in docker_outer:
            message = docker_outer.get("log", "").strip()
    except json.JSONDecodeError:
        pass

    parsed = {}

    # 2. Find and extract the embedded JSON payload at the end of the string
    json_start_idx = message.find('{')

    if json_start_idx != -1:
        json_str = message[json_start_idx:]
        try:
            decoded = json.loads(json_str)
            if isinstance(decoded, dict):
                parsed = decoded
        except json.JSONDecodeError:
            pass  # If it's not valid JSON, we fallback below

    # 3. Extract the Log Level manually since it is in the text part, not the JSON
    if "level" not in parsed:
        parts = message.split('\t')
        if len(parts) >= 2:
            # Log format usually puts level at index 1: [0] Timestamp, [1] Level
            parsed["level"] = parts[1].strip()
        else:
            # Fallback text matching
            upper_msg = message.upper()
            for lvl in ["ERROR", "WARN", "PANIC", "FATAL"]:
                if lvl in upper_msg:
                    parsed["level"] = lvl
                    break

    # 4. Clean up the 'message' field so the email is highly readable
    text_prefix = message[:json_start_idx].strip() if json_start_idx != -1 else message
    parsed["msg"] = text_prefix

    # 5. Add CloudWatch metadata
    parsed["timestamp_ms"] = event.get("timestamp", 0)
    parsed["log_stream_name"] = event.get("logStreamName", "")
    parsed["event_id"] = event.get("eventId", "")
    parsed["raw_message"] = raw_message

    return parsed


def is_problem_log(log):
    level = str(log.get("level", "")).lower()
    if level in ERROR_LEVELS:
        return True

    try:
        return int(log.get("status", 0)) >= 500
    except (TypeError, ValueError):
        return False


def event_key(log):
    if log.get("event_id"):
        return log["event_id"]
    return "|".join([
        str(log.get("timestamp_ms", "")),
        str(log.get("log_stream_name", "")),
        str(log.get("raw_message", "")),
    ])


def problem_sort_key(log):
    return (
        problem_priority(log),
        int(log.get("duration_ms") or 0),
        int(log.get("timestamp_ms") or 0),
    )


def problem_priority(log):
    if bool(log.get("timeout")):
        return 4
    try:
        if int(log.get("status", 0)) >= 500:
            return 3
    except (TypeError, ValueError):
        pass
    level = str(log.get("level", "")).lower()
    if level in {"error", "fatal", "panic", "dpanic"}:
        return 2
    if level in {"warn", "warning"}:
        return 1
    return 0


def build_message(event, service, log_group, recent_logs):
    detail = event.get("detail", {})
    state = detail.get("state", {})
    previous_state = detail.get("previousState", {})

    lines = [
        "Smatch incident alarm",
        "",
        f"Service: {service}",
        f"Alarm: {detail.get('alarmName', 'unknown')}",
        f"State: {previous_state.get('value', 'unknown')} -> {state.get('value', 'unknown')}",
        f"Reason: {state.get('reason', 'n/a')}",
        f"Alarm time: {state.get('timestamp', event.get('time', 'n/a'))}",
        f"Region: {event.get('region', 'n/a')}",
        f"Log group: {log_group or 'not mapped'}",
        f"Lookback: last {LOOKBACK_MINUTES} minutes",
        "",
        "Problem summary:",
    ]

    if not recent_logs:
        lines.append("No timeout, warn, error, panic, or 5xx logs found in the lookback window.")
        return "\n".join(lines)

    summary = summarize_logs(recent_logs)
    lines.extend([
        f"Total included: {len(recent_logs)}",
        f"Timeouts: {summary['timeouts']}",
        f"5xx: {summary['5xx']}",
        f"Warn/Error/Panic: {summary['warn_error']}",
        "",
        "Recent problem logs:",
    ])

    for log in recent_logs:
        lines.extend(format_log(log))

    return "\n".join(lines)


def summarize_logs(logs):
    summary = {"timeouts": 0, "5xx": 0, "warn_error": 0}
    for log in logs:
        if bool(log.get("timeout")):
            summary["timeouts"] += 1
        try:
            if int(log.get("status", 0)) >= 500:
                summary["5xx"] += 1
        except (TypeError, ValueError):
            pass
        if str(log.get("level", "")).lower() in ERROR_LEVELS:
            summary["warn_error"] += 1
    return summary


def format_log(log):
    ts = datetime.fromtimestamp(log.get("timestamp_ms", 0) / 1000, tz=timezone.utc).isoformat()
    fields = [
        "",
        f"- time: {ts}",
        f"  stream: {log.get('log_stream_name', '')}",
        f"  level: {log.get('level', '')}",
        f"  message: {log.get('msg', '')}",
        f"  event: {log.get('event', '')}",
        f"  client_ip: {log.get('client_ip', '')}",
        f"  endpoint: {log.get('endpoint', log.get('path', ''))}",
        f"  route: {log.get('route', '')}",
        f"  method: {log.get('method', '')}",
        f"  status: {log.get('status', '')}",
        f"  outcome: {log.get('outcome', '')}",
        f"  timeout: {log.get('timeout', '')}",
        f"  duration_ms: {log.get('duration_ms', '')}",
        f"  latency_budget_ms: {log.get('latency_budget_ms', '')}",
        f"  request_id: {log.get('request_id', '')}",
    ]
    if log.get("error"):
        fields.append(f"  error: {log.get('error')}")
    if log.get("panic"):
        fields.append(f"  panic: {log.get('panic')}")
    return fields


def publish(alarm_name, service, message):
    subject = f"[smatch] {service} alarm: {alarm_name}"[:100]
    response = sns_client.publish(TopicArn=SNS_TOPIC_ARN, Subject=subject, Message=message)
    print(json.dumps({
        "level": "info",
        "msg": "published incident email",
        "alarm_name": alarm_name,
        "service": service,
        "message_id": response.get("MessageId", ""),
    }))
