# ── IAM Role for EC2 (ECR pull + SSM access) ─────────────────────────────────

resource "aws_iam_role" "backend" {
  name = "${var.app_name}-ec2-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = { Name = "${var.app_name}-ec2-role" }
}

resource "aws_iam_role_policy_attachment" "ecr_readonly" {
  role       = aws_iam_role.backend.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy_attachment" "ssm_core" {
  role       = aws_iam_role.backend.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy" "backend_firebase_secret" {
  name = "firebase-secret-read"
  role = aws_iam_role.backend.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue"]
      Resource = "arn:aws:secretsmanager:${var.aws_region}:*:secret:smatch/firebase/credentials*"
    }]
  })
}

resource "aws_iam_role_policy" "backend_s3_access" {
  name = "s3-runtime-access"
  role = aws_iam_role.backend.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.profile.arn,
          aws_s3_bucket.matches.arn,
          aws_s3_bucket.business_docs.arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "s3:DeleteObject",
          "s3:GetObject",
          "s3:PutObject"
        ]
        Resource = [
          "${aws_s3_bucket.profile.arn}/*",
          "${aws_s3_bucket.matches.arn}/*",
          "${aws_s3_bucket.business_docs.arn}/*"
        ]
      }
    ]
  })
}

resource "aws_iam_instance_profile" "backend" {
  name = "${var.app_name}-ec2-profile"
  role = aws_iam_role.backend.name
}

# ── Launch Template ───────────────────────────────────────────────────────────

resource "aws_launch_template" "backend" {
  name_prefix   = "${var.app_name}-lt-"
  image_id      = var.ami_id
  instance_type = var.instance_type

  vpc_security_group_ids = [aws_security_group.backend.id]

  iam_instance_profile {
    arn = aws_iam_instance_profile.backend.arn
  }

  monitoring {
    enabled = true
  }

  user_data = base64encode(templatefile("${path.module}/user_data.sh.tpl", {
    aws_region                                     = var.aws_region
    ecr_repo_url                                   = var.ecr_repo_url
    image_tag                                      = "latest"
    backend_port                                   = var.backend_port
    database_url                                   = "postgresql://${var.db_username}:${var.db_password}@${aws_db_instance.main.address}:${aws_db_instance.main.port}/${var.db_name}?sslmode=require"
    redis_host                                     = aws_elasticache_cluster.main.cache_nodes[0].address
    redis_port                                     = tostring(aws_elasticache_cluster.main.port)
    redis_password                                 = var.redis_password
    s3_bucket_profile                              = aws_s3_bucket.profile.bucket
    s3_bucket_matches                              = aws_s3_bucket.matches.bucket
    s3_bucket_business_docs                        = aws_s3_bucket.business_docs.bucket
    zalopay_app_id                                 = var.zalopay_app_id
    zalopay_key1                                   = var.zalopay_key1
    zalopay_key2                                   = var.zalopay_key2
    zalopay_endpoint                               = var.zalopay_endpoint
    zalopay_callback_url                           = var.zalopay_callback_url
    tile_server_url                                = var.tile_server_url
    admin_secret                                   = var.admin_secret
    admin_web_origin                               = var.admin_domain_name != "" ? "https://${var.admin_domain_name}" : ""
    rate_limit_trusted_ips                         = var.rate_limit_trusted_ips
    load_test_stress_enabled                       = tostring(var.load_test_stress_enabled)
    cloudwatch_log_group_name                      = aws_cloudwatch_log_group.backend.name
    service_name                                   = "backend"
    container_cpu_limit                            = tostring(var.backend_container_cpu_limit)
    cloudwatch_cpu_metrics_enabled                 = true
    cloudwatch_metrics_collection_interval_seconds = var.backend_cloudwatch_metrics_collection_interval_seconds
  }))

  lifecycle {
    create_before_destroy = true
  }

  tags = { Name = "${var.app_name}-lt" }
}

# ── Auto Scaling Group ────────────────────────────────────────────────────────
# Instances are placed in the private app subnets (no public IP, egress via NAT).

resource "aws_autoscaling_group" "backend" {
  name                = "${var.app_name}-asg"
  min_size            = var.asg_min_size
  max_size            = var.asg_max_size
  desired_capacity    = var.asg_desired_capacity
  vpc_zone_identifier = aws_subnet.private_app[*].id

  launch_template {
    id      = aws_launch_template.backend.id
    version = aws_launch_template.backend.latest_version
  }

  target_group_arns         = [aws_lb_target_group.backend.arn]
  health_check_type         = "ELB"
  health_check_grace_period = 120

  instance_refresh {
    strategy = "Rolling"

    preferences {
      min_healthy_percentage = 50
    }
  }

  lifecycle {
    ignore_changes = [desired_capacity]
  }

  dynamic "tag" {
    for_each = {
      Name    = "${var.app_name}-backend"
      Env     = var.environment
      Service = "backend"
    }
    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = true
    }
  }
}

resource "aws_autoscaling_policy" "backend_cpu_target" {
  name                   = "${var.app_name}-${var.environment}-backend-cpu-target"
  autoscaling_group_name = aws_autoscaling_group.backend.name
  policy_type            = "TargetTrackingScaling"

  estimated_instance_warmup = 120

  target_tracking_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ASGAverageCPUUtilization"
    }

    target_value = var.asg_cpu_target_percent
  }
}

resource "aws_autoscaling_policy" "backend_alb_request_target" {
  name                   = "${var.app_name}-${var.environment}-backend-alb-request-target"
  autoscaling_group_name = aws_autoscaling_group.backend.name
  policy_type            = "TargetTrackingScaling"

  estimated_instance_warmup = 120

  target_tracking_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ALBRequestCountPerTarget"
      resource_label         = "${aws_lb.main.arn_suffix}/${aws_lb_target_group.backend.arn_suffix}"
    }

    target_value = var.asg_request_count_target_per_minute
  }
}
