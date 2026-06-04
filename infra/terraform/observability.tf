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
  timeout          = 60
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

# ── CPU alarms and EventBridge routing ───────────────────────────────────────

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

resource "aws_cloudwatch_event_rule" "cpu_alarm_to_lambda" {
  name        = "${var.app_name}-${var.environment}-cpu-alarm-to-lambda"
  description = "Routes backend/admin CPU alarm state changes to the log notifier Lambda."

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

  tags = { Name = "${var.app_name}-cpu-alarm-to-lambda" }
}

resource "aws_cloudwatch_event_target" "cpu_alarm_to_lambda" {
  rule      = aws_cloudwatch_event_rule.cpu_alarm_to_lambda.name
  target_id = "log-alarm-notifier"
  arn       = aws_lambda_function.log_alarm_notifier.arn
}

resource "aws_lambda_permission" "allow_eventbridge_cpu_alarm" {
  statement_id  = "AllowExecutionFromEventBridgeCpuAlarm"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.log_alarm_notifier.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.cpu_alarm_to_lambda.arn
}
