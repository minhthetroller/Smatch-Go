# ── CloudWatch Logs for application instances ────────────────────────────────

resource "aws_cloudwatch_log_group" "backend" {
  name              = "/smatch/${var.environment}/backend"
  retention_in_days = var.log_retention_days

  tags = { Name = "${var.app_name}-backend-logs" }
}

resource "aws_cloudwatch_log_group" "admin" {
  name              = "/smatch/${var.environment}/admin"
  retention_in_days = var.log_retention_days

  tags = { Name = "${var.app_name}-admin-logs" }
}

resource "aws_iam_role_policy_attachment" "backend_cloudwatch_agent" {
  role       = aws_iam_role.backend.name
  policy_arn = "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"
}

resource "aws_iam_role_policy_attachment" "admin_cloudwatch_agent" {
  role       = aws_iam_role.admin.name
  policy_arn = "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"
}

# ── SNS incident topic ───────────────────────────────────────────────────────

locals {
  log_alarm_notifier_timeout_seconds = 600
}

resource "aws_sns_topic" "incident_alerts" {
  name = "${var.app_name}-${var.environment}-incident-alerts"

  tags = { Name = "${var.app_name}-incident-alerts" }
}

resource "aws_sns_topic_subscription" "incident_email" {
  topic_arn = aws_sns_topic.incident_alerts.arn
  protocol  = "email"
  endpoint  = var.incident_email
}

# ── Lambda notifier ─────────────────────────────────────────────────────────

data "archive_file" "log_alarm_notifier" {
  type        = "zip"
  source_dir  = "${path.module}/../lambda/log_alarm_notifier"
  output_path = "${path.module}/.terraform/log_alarm_notifier.zip"
  excludes    = ["__pycache__/*", "*.pyc"]
}

resource "aws_iam_role" "log_alarm_notifier" {
  name = "${var.app_name}-${var.environment}-log-alarm-notifier-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = { Name = "${var.app_name}-log-alarm-notifier-role" }
}

resource "aws_iam_role_policy_attachment" "log_alarm_notifier_basic" {
  role       = aws_iam_role.log_alarm_notifier.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "log_alarm_notifier" {
  name = "${var.app_name}-${var.environment}-log-alarm-notifier"
  role = aws_iam_role.log_alarm_notifier.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:FilterLogEvents",
          "logs:DescribeLogStreams",
          "logs:DescribeLogGroups"
        ]
        Resource = [
          aws_cloudwatch_log_group.backend.arn,
          "${aws_cloudwatch_log_group.backend.arn}:*",
          aws_cloudwatch_log_group.admin.arn,
          "${aws_cloudwatch_log_group.admin.arn}:*"
        ]
      },
      {
        Effect   = "Allow"
        Action   = ["sns:Publish"]
        Resource = aws_sns_topic.incident_alerts.arn
      },
      {
        Effect = "Allow"
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes",
          "sqs:ChangeMessageVisibility"
        ]
        Resource = aws_sqs_queue.incident_alarm.arn
      }
    ]
  })
}

resource "aws_lambda_function" "log_alarm_notifier" {
  function_name    = "${var.app_name}-${var.environment}-log-alarm-notifier"
  description      = "Queries recent app logs for CPU alarms and publishes incident email context."
  role             = aws_iam_role.log_alarm_notifier.arn
  handler          = "app.handler"
  runtime          = "python3.12"
  timeout          = local.log_alarm_notifier_timeout_seconds
  memory_size      = 256
  filename         = data.archive_file.log_alarm_notifier.output_path
  source_code_hash = data.archive_file.log_alarm_notifier.output_base64sha256

  environment {
    variables = {
      SNS_TOPIC_ARN = aws_sns_topic.incident_alerts.arn
      LOG_GROUPS_JSON = jsonencode({
        backend = aws_cloudwatch_log_group.backend.name
        admin   = aws_cloudwatch_log_group.admin.name
      })
      ALARM_SERVICE_MAP_JSON = jsonencode({
        (aws_cloudwatch_metric_alarm.backend_cpu_high.alarm_name) = "backend"
        (aws_cloudwatch_metric_alarm.admin_cpu_high.alarm_name)   = "admin"
      })
      LOOKBACK_MINUTES = tostring(var.lambda_log_lookback_minutes)
    }
  }

  tags = { Name = "${var.app_name}-log-alarm-notifier" }
}

# ── Delayed incident alarm queue ─────────────────────────────────────────────

resource "aws_sqs_queue" "incident_alarm_dlq" {
  name                      = "${var.app_name}-${var.environment}-incident-alarm-dlq"
  message_retention_seconds = 1209600

  tags = { Name = "${var.app_name}-incident-alarm-dlq" }
}

resource "aws_sqs_queue" "incident_alarm" {
  name                       = "${var.app_name}-${var.environment}-incident-alarm-delay"
  delay_seconds              = var.incident_alarm_queue_delay_seconds
  visibility_timeout_seconds = var.incident_alarm_queue_visibility_timeout_seconds
  message_retention_seconds  = 1209600

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.incident_alarm_dlq.arn
    maxReceiveCount     = 5
  })

  lifecycle {
    precondition {
      condition     = var.incident_alarm_queue_visibility_timeout_seconds >= local.log_alarm_notifier_timeout_seconds * 6
      error_message = "incident_alarm_queue_visibility_timeout_seconds must be at least six times the Lambda timeout for SQS event source mappings."
    }
  }

  tags = { Name = "${var.app_name}-incident-alarm-delay" }
}

data "aws_iam_policy_document" "incident_alarm_queue" {
  statement {
    sid     = "AllowEventBridgeIncidentAlarmMessages"
    effect  = "Allow"
    actions = ["sqs:SendMessage"]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }

    resources = [aws_sqs_queue.incident_alarm.arn]

    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_cloudwatch_event_rule.cpu_alarm_to_queue.arn]
    }
  }
}

resource "aws_sqs_queue_policy" "incident_alarm" {
  queue_url = aws_sqs_queue.incident_alarm.id
  policy    = data.aws_iam_policy_document.incident_alarm_queue.json
}

resource "aws_lambda_event_source_mapping" "log_alarm_notifier_sqs" {
  event_source_arn        = aws_sqs_queue.incident_alarm.arn
  function_name           = aws_lambda_function.log_alarm_notifier.arn
  batch_size              = 1
  function_response_types = ["ReportBatchItemFailures"]

  depends_on = [aws_iam_role_policy.log_alarm_notifier]
}

# ── CPU alarms and delayed EventBridge routing ───────────────────────────────

resource "aws_cloudwatch_metric_alarm" "backend_cpu_high" {
  alarm_name          = "${var.app_name}-${var.environment}-backend-cpu-high"
  alarm_description   = "Backend ASG average CPU is at or above ${var.cpu_alarm_threshold}%."
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = var.cpu_alarm_evaluation_periods
  datapoints_to_alarm = var.cpu_alarm_evaluation_periods
  metric_name         = "CPUUtilization"
  namespace           = "AWS/EC2"
  period              = var.cpu_alarm_period_seconds
  statistic           = "Average"
  threshold           = var.cpu_alarm_threshold
  treat_missing_data  = "notBreaching"

  dimensions = {
    AutoScalingGroupName = aws_autoscaling_group.backend.name
  }

  tags = { Name = "${var.app_name}-backend-cpu-high" }
}

resource "aws_cloudwatch_metric_alarm" "admin_cpu_high" {
  alarm_name          = "${var.app_name}-${var.environment}-admin-cpu-high"
  alarm_description   = "Admin ASG average CPU is at or above ${var.cpu_alarm_threshold}%."
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = var.cpu_alarm_evaluation_periods
  datapoints_to_alarm = var.cpu_alarm_evaluation_periods
  metric_name         = "CPUUtilization"
  namespace           = "AWS/EC2"
  period              = var.cpu_alarm_period_seconds
  statistic           = "Average"
  threshold           = var.cpu_alarm_threshold
  treat_missing_data  = "notBreaching"

  dimensions = {
    AutoScalingGroupName = aws_autoscaling_group.admin.name
  }

  tags = { Name = "${var.app_name}-admin-cpu-high" }
}

resource "aws_cloudwatch_event_rule" "cpu_alarm_to_queue" {
  name        = "${var.app_name}-${var.environment}-cpu-alarm-to-queue"
  description = "Routes backend/admin CPU alarm state changes to the delayed incident queue."

  event_pattern = jsonencode({
    source        = ["aws.cloudwatch"]
    "detail-type" = ["CloudWatch Alarm State Change"]
    detail = {
      alarmName = [
        aws_cloudwatch_metric_alarm.backend_cpu_high.alarm_name,
        aws_cloudwatch_metric_alarm.admin_cpu_high.alarm_name
      ]
      state = {
        value = ["ALARM"]
      }
    }
  })

  tags = { Name = "${var.app_name}-cpu-alarm-to-queue" }
}

resource "aws_cloudwatch_event_target" "cpu_alarm_to_queue" {
  rule      = aws_cloudwatch_event_rule.cpu_alarm_to_queue.name
  target_id = "incident-alarm-delay-queue"
  arn       = aws_sqs_queue.incident_alarm.arn
}
