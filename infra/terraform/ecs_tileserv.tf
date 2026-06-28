# ── pg_tileserv on ECS Fargate ────────────────────────────────────────────────

locals {
  tileserv_database_url = "postgresql://${var.db_username}:${var.db_password}@${aws_db_instance.replica.address}:${var.db_port}/${var.db_name}?sslmode=require"
}

resource "aws_cloudwatch_log_group" "tileserv" {
  name              = "/smatch/${var.environment}/tileserv"
  retention_in_days = var.log_retention_days

  tags = { Name = "${var.app_name}-tileserv-logs" }
}

resource "aws_ecs_cluster" "tileserv" {
  name = "${var.app_name}-tileserv"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = { Name = "${var.app_name}-tileserv-cluster" }
}

resource "aws_iam_role" "tileserv_task_execution" {
  name = "${var.app_name}-tileserv-task-execution-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = { Name = "${var.app_name}-tileserv-task-execution-role" }
}

resource "aws_iam_role_policy_attachment" "tileserv_task_execution" {
  role       = aws_iam_role.tileserv_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role" "tileserv_task" {
  name = "${var.app_name}-tileserv-task-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = { Name = "${var.app_name}-tileserv-task-role" }
}

resource "aws_ecs_task_definition" "tileserv" {
  family                   = "${var.app_name}-tileserv"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(var.tileserv_task_cpu)
  memory                   = tostring(var.tileserv_task_memory)
  execution_role_arn       = aws_iam_role.tileserv_task_execution.arn
  task_role_arn            = aws_iam_role.tileserv_task.arn

  container_definitions = jsonencode([
    {
      name      = "pg-tileserv"
      image     = var.tileserv_image_url
      essential = true
      portMappings = [{
        containerPort = var.tileserv_port
        hostPort      = var.tileserv_port
        protocol      = "tcp"
      }]
      environment = [
        { name = "DATABASE_URL", value = local.tileserv_database_url },
        { name = "TS_HTTPPORT", value = tostring(var.tileserv_port) },
        { name = "TS_BASEPATH", value = "/" }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.tileserv.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "pg_tileserv"
        }
      }
    }
  ])

  tags = { Name = "${var.app_name}-tileserv-task" }
}

resource "aws_ecs_service" "tileserv" {
  name            = "${var.app_name}-tileserv"
  cluster         = aws_ecs_cluster.tileserv.id
  task_definition = aws_ecs_task_definition.tileserv.arn
  desired_count   = var.tileserv_desired_count
  launch_type     = "FARGATE"

  health_check_grace_period_seconds = 120

  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200

  network_configuration {
    subnets          = aws_subnet.private_app[*].id
    security_groups  = [aws_security_group.tileserv.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.tileserv.arn
    container_name   = "pg-tileserv"
    container_port   = var.tileserv_port
  }

  depends_on = [
    aws_lb_listener_rule.tileserv,
    aws_lb_listener_rule.tileserv_http,
  ]

  tags = { Name = "${var.app_name}-tileserv-service" }
}
