# ── IAM Role for Admin EC2 ────────────────────────────────────────────────────

resource "aws_iam_role" "admin" {
  name = "${var.app_name}-admin-ec2-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = { Name = "${var.app_name}-admin-ec2-role" }
}

resource "aws_iam_role_policy_attachment" "admin_ecr_readonly" {
  role       = aws_iam_role.admin.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

resource "aws_iam_role_policy_attachment" "admin_ssm_core" {
  role       = aws_iam_role.admin.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy" "admin_firebase_secret" {
  name = "firebase-secret-read"
  role = aws_iam_role.admin.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue"]
      Resource = "arn:aws:secretsmanager:${var.aws_region}:*:secret:smatch/firebase/credentials*"
    }]
  })
}

resource "aws_iam_instance_profile" "admin" {
  name = "${var.app_name}-admin-ec2-profile"
  role = aws_iam_role.admin.name
}

# ── Launch Template ───────────────────────────────────────────────────────────

resource "aws_launch_template" "admin" {
  name_prefix   = "${var.app_name}-admin-lt-"
  image_id      = var.ami_id
  instance_type = var.instance_type

  vpc_security_group_ids = [aws_security_group.admin.id]

  iam_instance_profile {
    arn = aws_iam_instance_profile.admin.arn
  }

  user_data = base64encode(templatefile("${path.module}/user_data.sh.tpl", {
    aws_region             = var.aws_region
    ecr_repo_url           = var.ecr_repo_url
    image_tag              = "admin"
    backend_port           = var.backend_port
    database_url           = "postgresql://${var.db_username}:${var.db_password}@${aws_db_instance.main.address}:${aws_db_instance.main.port}/${var.db_name}?sslmode=require"
    redis_host             = aws_elasticache_cluster.main.cache_nodes[0].address
    redis_port             = tostring(aws_elasticache_cluster.main.port)
    redis_password         = var.redis_password
    s3_bucket_profile      = aws_s3_bucket.profile.bucket
    s3_bucket_matches      = aws_s3_bucket.matches.bucket
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

  tags = { Name = "${var.app_name}-admin-lt" }
}

# ── Auto Scaling Group ────────────────────────────────────────────────────────

resource "aws_autoscaling_group" "admin" {
  name                = "${var.app_name}-admin-asg"
  min_size            = var.admin_asg_min_size
  max_size            = var.admin_asg_max_size
  desired_capacity    = var.admin_asg_desired_capacity
  vpc_zone_identifier = aws_subnet.private_app[*].id

  launch_template {
    id      = aws_launch_template.admin.id
    version = "$Latest"
  }

  target_group_arns         = [aws_lb_target_group.admin.arn]
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
      Name = "${var.app_name}-admin"
      Env  = var.environment
    }
    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = true
    }
  }
}
