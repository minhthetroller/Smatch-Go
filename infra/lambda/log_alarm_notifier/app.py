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


def handler(event, _context):
    detail = event.get("detail", {})
    alarm_name = detail.get("alarmName", "unknown-alarm")
    service = ALARM_SERVICE_MAP.get(alarm_name, infer_service(alarm_name))
    log_group = LOG_GROUPS.get(service)

    if not log_group:
        message = build_message(event, service, None, [])
        publish(alarm_name, service, message)
        return {"published": True, "service": service, "log_group_found": False}

    recent_logs = find_recent_problem_logs(log_group)
    message = build_message(event, service, log_group, recent_logs)
    publish(alarm_name, service, message)
    return {
        "published": True,
        "service": service,
        "log_group": log_group,
        "log_count": len(recent_logs),
    }


def infer_service(alarm_name):
    lowered = alarm_name.lower()
    if "admin" in lowered:
        return "admin"
    return "backend"


def find_recent_problem_logs(log_group):
    end_ms = int(time.time() * 1000)
    start_ms = end_ms - (LOOKBACK_MINUTES * 60 * 1000)
    events = []
    next_token = None

    while len(events) < MAX_LOG_EVENTS:
        request = {
            "logGroupName": log_group,
            "startTime": start_ms,
            "endTime": end_ms,
            "limit": min(100, MAX_LOG_EVENTS * 4),
        }
        if next_token:
            request["nextToken"] = next_token

        response = logs_client.filter_log_events(**request)
        for raw_event in response.get("events", []):
            parsed = parse_log_event(raw_event)
            if is_problem_log(parsed):
                events.append(parsed)
                if len(events) >= MAX_LOG_EVENTS:
                    break

        next_token = response.get("nextToken")
        if not next_token:
            break

    return sorted(events, key=lambda item: item.get("timestamp_ms", 0), reverse=True)[:MAX_LOG_EVENTS]


def parse_log_event(event):
    raw_message = event.get("message", "")
    message = raw_message

    try:
        docker_outer = json.loads(raw_message)
        if isinstance(docker_outer, dict) and "log" in docker_outer:
            message = docker_outer.get("log", "").strip()
    except json.JSONDecodeError:
        pass

    parsed = {}
    try:
        decoded = json.loads(message)
        if isinstance(decoded, dict):
            parsed = decoded
    except json.JSONDecodeError:
        parsed = {"msg": message}

    parsed["timestamp_ms"] = event.get("timestamp", 0)
    parsed["log_stream_name"] = event.get("logStreamName", "")
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
        "Recent problem logs:",
    ]

    if not recent_logs:
        lines.append("No warn/error/5xx logs found in the lookback window.")
        return "\n".join(lines)

    for log in recent_logs:
        lines.extend(format_log(log))

    return "\n".join(lines)


def format_log(log):
    ts = datetime.fromtimestamp(log.get("timestamp_ms", 0) / 1000, tz=timezone.utc).isoformat()
    fields = [
        "",
        f"- time: {ts}",
        f"  stream: {log.get('log_stream_name', '')}",
        f"  level: {log.get('level', '')}",
        f"  message: {log.get('msg', '')}",
        f"  client_ip: {log.get('client_ip', '')}",
        f"  endpoint: {log.get('endpoint', log.get('path', ''))}",
        f"  method: {log.get('method', '')}",
        f"  status: {log.get('status', '')}",
        f"  duration_ms: {log.get('duration_ms', '')}",
    ]
    if log.get("error"):
        fields.append(f"  error: {log.get('error')}")
    return fields


def publish(alarm_name, service, message):
    subject = f"[smatch] {service} alarm: {alarm_name}"[:100]
    sns_client.publish(TopicArn=SNS_TOPIC_ARN, Subject=subject, Message=message)
