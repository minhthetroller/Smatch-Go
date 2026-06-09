import importlib
import json
import os
import sys
import types
import unittest


class FakeLogsClient:
    def __init__(self):
        self.requests = []
        self.responses = []

    def filter_log_events(self, **kwargs):
        self.requests.append(kwargs)
        if self.responses:
            return self.responses.pop(0)
        return {"events": []}


class FakeSNSClient:
    def __init__(self):
        self.published = []

    def publish(self, **kwargs):
        self.published.append(kwargs)
        return {"MessageId": "m-1"}


class LambdaAppTest(unittest.TestCase):
    def setUp(self):
        self.logs = FakeLogsClient()
        self.sns = FakeSNSClient()
        fake_boto3 = types.SimpleNamespace(
            client=lambda name: self.logs if name == "logs" else self.sns
        )
        sys.modules["boto3"] = fake_boto3
        os.environ["SNS_TOPIC_ARN"] = "arn:sns"
        os.environ["LOG_GROUPS_JSON"] = json.dumps({
            "backend": "/logs/backend",
            "admin": "/logs/admin",
        })
        os.environ["ALARM_SERVICE_MAP_JSON"] = json.dumps({"backend-cpu": "backend"})
        os.environ["ALARM_SERVICE_PREFIXES_JSON"] = json.dumps({
            "TargetTracking-smatch-asg-AlarmHigh-": "backend",
            "TargetTracking-smatch-admin-asg-AlarmHigh-": "admin",
        })
        os.environ["LOOKBACK_MINUTES"] = "15"
        os.environ["MAX_LOG_EVENTS"] = "10"
        sys.modules.pop("app", None)
        self.app = importlib.import_module("app")
        self.app.logs_client = self.logs
        self.app.sns_client = self.sns

    def test_parse_log_event_unwraps_docker_json(self):
        inner = {
            "level": "error",
            "msg": "http request completed",
            "event": "http_request",
            "status": 504,
            "timeout": True,
        }
        raw = {"log": json.dumps(inner) + "\n"}

        parsed = self.app.parse_log_event({
            "message": json.dumps(raw),
            "timestamp": 123,
            "logStreamName": "i-1/backend",
            "eventId": "e-1",
        })

        self.assertEqual(parsed["event"], "http_request")
        self.assertEqual(parsed["status"], 504)
        self.assertTrue(parsed["timeout"])
        self.assertEqual(parsed["event_id"], "e-1")

    def test_find_recent_problem_logs_uses_targeted_filters_and_dedupes(self):
        timeout_log = {
            "level": "error",
            "msg": "http request completed",
            "event": "http_request",
            "status": 504,
            "timeout": True,
            "duration_ms": 5001,
            "request_id": "r-1",
        }
        event = {
            "eventId": "same-event",
            "timestamp": 1000,
            "logStreamName": "i-1/backend",
            "message": json.dumps(timeout_log),
        }
        self.logs.responses = [
            {"events": [event]},
            {"events": [event]},
            {"events": []},
            {"events": []},
            {"events": []},
            {"events": []},
        ]

        logs = self.app.find_recent_problem_logs("/logs/backend")

        self.assertEqual(len(logs), 1)
        self.assertTrue(logs[0]["timeout"])
        filter_patterns = [r.get("filterPattern") for r in self.logs.requests]
        self.assertIn('{ $.event = "http_request" && $.timeout = true }', filter_patterns)

    def test_build_message_includes_summary_and_request_fields(self):
        event = {
            "region": "ap-southeast-1",
            "time": "2026-06-07T00:00:00Z",
            "detail": {
                "alarmName": "backend-cpu",
                "state": {"value": "ALARM", "reason": "CPU high"},
                "previousState": {"value": "OK"},
            },
        }
        message = self.app.build_message(event, "backend", "/logs/backend", [{
            "timestamp_ms": 1000,
            "level": "error",
            "msg": "http request completed",
            "event": "http_request",
            "endpoint": "/api/courts",
            "route": "/api/courts",
            "method": "GET",
            "status": 504,
            "outcome": "timeout",
            "timeout": True,
            "duration_ms": 5001,
            "latency_budget_ms": 5000,
            "request_id": "r-1",
        }])

        self.assertIn("Service: backend", message)
        self.assertIn("Problem summary:", message)
        self.assertIn("Timeouts: 1", message)
        self.assertIn("latency_budget_ms: 5000", message)
        self.assertIn("request_id: r-1", message)

    def test_handler_publishes_message(self):
        self.logs.responses = [{"events": []} for _ in range(6)]
        result = self.app.handler({
            "region": "ap-southeast-1",
            "detail": {
                "alarmName": "backend-cpu",
                "state": {"value": "ALARM"},
                "previousState": {"value": "OK"},
            },
        }, None)

        self.assertTrue(result["published"])
        self.assertEqual(result["service"], "backend")
        self.assertEqual(len(self.sns.published), 1)
        self.assertIn("No timeout, warn, error, panic, or 5xx logs", self.sns.published[0]["Message"])

    def test_handler_processes_sqs_wrapped_alarm_event(self):
        self.logs.responses = [{"events": []} for _ in range(6)]
        eventbridge_event = {
            "region": "ap-southeast-1",
            "detail": {
                "alarmName": "backend-cpu",
                "state": {"value": "ALARM"},
                "previousState": {"value": "OK"},
            },
        }

        result = self.app.handler({
            "Records": [{
                "messageId": "msg-1",
                "eventSource": "aws:sqs",
                "body": json.dumps(eventbridge_event),
            }],
        }, None)

        self.assertEqual(result, {"batchItemFailures": []})
        self.assertEqual(len(self.sns.published), 1)
        self.assertIn("Alarm: backend-cpu", self.sns.published[0]["Message"])

    def test_handler_returns_partial_batch_failure_for_bad_sqs_body(self):
        result = self.app.handler({
            "Records": [{
                "messageId": "msg-1",
                "eventSource": "aws:sqs",
                "body": json.dumps({"notDetail": True}),
            }],
        }, None)

        self.assertEqual(result, {"batchItemFailures": [{"itemIdentifier": "msg-1"}]})
        self.assertEqual(len(self.sns.published), 0)

    def test_resolve_service_uses_target_tracking_prefixes(self):
        self.assertEqual(
            self.app.resolve_service("TargetTracking-smatch-asg-AlarmHigh-abc123"),
            "backend",
        )
        self.assertEqual(
            self.app.resolve_service("TargetTracking-smatch-admin-asg-AlarmHigh-def456"),
            "admin",
        )


if __name__ == "__main__":
    unittest.main()
