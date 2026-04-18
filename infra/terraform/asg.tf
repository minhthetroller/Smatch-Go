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

  user_data = base64encode(templatefile("${path.module}/user_data.sh.tpl", {
    aws_region             = var.aws_region
    ecr_repo_url           = var.ecr_repo_url
    backend_port           = var.backend_port
    database_url           = "postgresql://${var.db_username}:${var.db_password}@${aws_db_instance.main.address}:${aws_db_instance.main.port}/${var.db_name}?sslmode=require"
    redis_host             = aws_elasticache_cluster.main.cache_nodes[0].address
    redis_port             = tostring(aws_elasticache_cluster.main.port)
    redis_password         = var.redis_password
    s3_bucket_profile      = aws_s3_bucket.profile.bucket
    s3_bucket_matches      = aws_s3_bucket.matches.bucket
    firebase_creds_file    = var.firebase_credentials_file
    zalopay_app_id         = var.zalopay_app_id
    zalopay_key1           = var.zalopay_key1
    zalopay_key2           = var.zalopay_key2
    zalopay_endpoint       = var.zalopay_endpoint
    zalopay_callback_url   = var.zalopay_callback_url
    tile_server_url        = var.tile_server_url
    admin_secret           = var.admin_secret
    rate_limit_trusted_ips = var.rate_limit_trusted_ips
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
    version = "$Latest"
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

  dynamic "tag" {
    for_each = {
      Name = "${var.app_name}-backend"
      Env  = var.environment
    }
    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = true
    }
  }
}
