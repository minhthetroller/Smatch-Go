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

# ── AWS Fault Injection Service CPU stress templates ─────────────────────────

locals {
  fis_backend_alb_load_url = local.api_fqdn != "" ? "https://${local.api_fqdn}/health" : "http://${aws_lb.main.dns_name}/health"
  fis_admin_alb_load_url   = var.admin_domain_name != "" ? "http://${aws_lb.main.dns_name}/api/admin/stats" : "http://${aws_lb.main.dns_name}/health"
}

resource "aws_iam_role" "fis" {
  name = "${var.app_name}-${var.environment}-fis-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "fis.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = { Name = "${var.app_name}-fis-role" }
}

resource "aws_iam_role_policy" "fis" {
  name = "${var.app_name}-${var.environment}-fis"
  role = aws_iam_role.fis.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ec2:DescribeInstances",
          "cloudwatch:DescribeAlarms",
          "ssm:SendCommand",
          "ssm:CancelCommand",
          "ssm:ListCommands",
          "ssm:ListCommandInvocations",
          "ssm:GetCommandInvocation",
          "ssm:DescribeInstanceInformation"
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_ssm_document" "fis_alb_http_load" {
  name          = "${var.app_name}-${var.environment}-fis-alb-http-load"
  document_type = "Command"

  content = jsonencode({
    schemaVersion = "2.2"
    description   = "Generate HTTP request load through the public ALB for FIS ASG tests."
    parameters = {
      TargetUrl = {
        type        = "String"
        description = "URL to request repeatedly."
      }
      HostHeader = {
        type        = "String"
        description = "Optional Host header for ALB listener rule testing."
        default     = ""
      }
      DurationSeconds = {
        type        = "String"
        description = "How long to generate load."
        default     = tostring(var.fis_alb_load_duration_seconds)
      }
      Concurrency = {
        type        = "String"
        description = "Number of parallel curl workers."
        default     = tostring(var.fis_alb_load_concurrency)
      }
    }
    mainSteps = [{
      action = "aws:runShellScript"
      name   = "runHttpLoad"
      inputs = {
        timeoutSeconds = "{{ DurationSeconds }}"
        runCommand = [
          <<-EOT
          #!/bin/sh
          set -eu

          target_url='{{ TargetUrl }}'
          host_header='{{ HostHeader }}'
          duration='{{ DurationSeconds }}'
          concurrency='{{ Concurrency }}'
          end_time=$(( $(date +%s) + duration ))

          worker() {
            while [ "$(date +%s)" -lt "$end_time" ]; do
              if [ -n "$host_header" ]; then
                curl -k -sS -o /dev/null -m 5 -H "Host: $host_header" "$target_url" || true
              else
                curl -k -sS -o /dev/null -m 5 "$target_url" || true
              fi
            done
          }

          i=0
          while [ "$i" -lt "$concurrency" ]; do
            worker &
            i=$((i + 1))
          done

          wait
          EOT
        ]
      }
    }]
  })

  tags = { Name = "${var.app_name}-fis-alb-http-load" }
}

resource "aws_fis_experiment_template" "backend_cpu_stress" {
  description = "Stress backend ASG CPU to validate CloudWatch/EventBridge/Lambda/SNS alerting."
  role_arn    = aws_iam_role.fis.arn

  stop_condition {
    source = "none"
  }

  target {
    name           = "BackendInstances"
    resource_type  = "aws:ec2:instance"
    selection_mode = "ALL"

    resource_tag {
      key   = "Service"
      value = "backend"
    }

    resource_tag {
      key   = "Env"
      value = var.environment
    }

    filter {
      path   = "State.Name"
      values = ["running"]
    }
  }

  action {
    name        = "RunCpuStress"
    description = "Run CPU stress on backend instances through SSM."
    action_id   = "aws:ssm:send-command"

    parameter {
      key   = "documentArn"
      value = "arn:aws:ssm:${var.aws_region}::document/AWSFIS-Run-CPU-Stress"
    }

    parameter {
      key   = "duration"
      value = "PT${var.fis_cpu_stress_duration_seconds}S"
    }

    parameter {
      key = "documentParameters"
      value = jsonencode({
        DurationSeconds     = tostring(var.fis_cpu_stress_duration_seconds)
        LoadPercent         = tostring(var.fis_cpu_stress_percent)
        InstallDependencies = "True"
      })
    }

    target {
      key   = "Instances"
      value = "BackendInstances"
    }
  }

  tags = { Name = "${var.app_name}-backend-cpu-stress" }
}

resource "aws_fis_experiment_template" "admin_cpu_stress" {
  description = "Stress admin ASG CPU to validate CloudWatch/EventBridge/Lambda/SNS alerting."
  role_arn    = aws_iam_role.fis.arn

  stop_condition {
    source = "none"
  }

  target {
    name           = "AdminInstances"
    resource_type  = "aws:ec2:instance"
    selection_mode = "ALL"

    resource_tag {
      key   = "Service"
      value = "admin"
    }

    resource_tag {
      key   = "Env"
      value = var.environment
    }

    filter {
      path   = "State.Name"
      values = ["running"]
    }
  }

  action {
    name        = "RunCpuStress"
    description = "Run CPU stress on admin instances through SSM."
    action_id   = "aws:ssm:send-command"

    parameter {
      key   = "documentArn"
      value = "arn:aws:ssm:${var.aws_region}::document/AWSFIS-Run-CPU-Stress"
    }

    parameter {
      key   = "duration"
      value = "PT${var.fis_cpu_stress_duration_seconds}S"
    }

    parameter {
      key = "documentParameters"
      value = jsonencode({
        DurationSeconds     = tostring(var.fis_cpu_stress_duration_seconds)
        LoadPercent         = tostring(var.fis_cpu_stress_percent)
        InstallDependencies = "True"
      })
    }

    target {
      key   = "Instances"
      value = "AdminInstances"
    }
  }

  tags = { Name = "${var.app_name}-admin-cpu-stress" }
}

resource "aws_fis_experiment_template" "backend_alb_http_load" {
  description = "Generate HTTP load through the ALB to validate backend ASG request-based scaling."
  role_arn    = aws_iam_role.fis.arn

  stop_condition {
    source = "none"
  }

  target {
    name           = "LoadGenerators"
    resource_type  = "aws:ec2:instance"
    selection_mode = "ALL"

    resource_tag {
      key   = "Service"
      value = "admin"
    }

    resource_tag {
      key   = "Env"
      value = var.environment
    }

    filter {
      path   = "State.Name"
      values = ["running"]
    }
  }

  action {
    name        = "RunAlbHttpLoad"
    description = "Generate backend ALB traffic from admin instances."
    action_id   = "aws:ssm:send-command"

    parameter {
      key   = "documentArn"
      value = aws_ssm_document.fis_alb_http_load.arn
    }

    parameter {
      key   = "duration"
      value = "PT${var.fis_alb_load_duration_seconds}S"
    }

    parameter {
      key = "documentParameters"
      value = jsonencode({
        TargetUrl       = local.fis_backend_alb_load_url
        HostHeader      = ""
        DurationSeconds = tostring(var.fis_alb_load_duration_seconds)
        Concurrency     = tostring(var.fis_alb_load_concurrency)
      })
    }

    target {
      key   = "Instances"
      value = "LoadGenerators"
    }
  }

  tags = { Name = "${var.app_name}-backend-alb-http-load" }
}

resource "aws_fis_experiment_template" "admin_alb_http_load" {
  description = "Generate HTTP load through the ALB to validate admin ASG request-based scaling."
  role_arn    = aws_iam_role.fis.arn

  stop_condition {
    source = "none"
  }

  target {
    name           = "LoadGenerators"
    resource_type  = "aws:ec2:instance"
    selection_mode = "ALL"

    resource_tag {
      key   = "Service"
      value = "backend"
    }

    resource_tag {
      key   = "Env"
      value = var.environment
    }

    filter {
      path   = "State.Name"
      values = ["running"]
    }
  }

  action {
    name        = "RunAlbHttpLoad"
    description = "Generate admin ALB traffic from backend instances."
    action_id   = "aws:ssm:send-command"

    parameter {
      key   = "documentArn"
      value = aws_ssm_document.fis_alb_http_load.arn
    }

    parameter {
      key   = "duration"
      value = "PT${var.fis_alb_load_duration_seconds}S"
    }

    parameter {
      key = "documentParameters"
      value = jsonencode({
        TargetUrl       = local.fis_admin_alb_load_url
        HostHeader      = var.admin_domain_name
        DurationSeconds = tostring(var.fis_alb_load_duration_seconds)
        Concurrency     = tostring(var.fis_alb_load_concurrency)
      })
    }

    target {
      key   = "Instances"
      value = "LoadGenerators"
    }
  }

  tags = { Name = "${var.app_name}-admin-alb-http-load" }
}
