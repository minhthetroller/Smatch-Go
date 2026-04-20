# ── IAM Role for pg_tileserv EC2 (SSM only — no ECR, binary installed at boot) ─

resource "aws_iam_role" "tileserv" {
  name = "${var.app_name}-tileserv-ec2-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = { Name = "${var.app_name}-tileserv-ec2-role" }
}

resource "aws_iam_role_policy_attachment" "tileserv_ssm_core" {
  role       = aws_iam_role.tileserv.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "tileserv" {
  name = "${var.app_name}-tileserv-ec2-profile"
  role = aws_iam_role.tileserv.name
}

# ── Launch Template ───────────────────────────────────────────────────────────

resource "aws_launch_template" "tileserv" {
  name_prefix   = "${var.app_name}-tileserv-lt-"
  image_id      = var.ami_id
  instance_type = var.instance_type

  vpc_security_group_ids = [aws_security_group.tileserv.id]

  iam_instance_profile {
    arn = aws_iam_instance_profile.tileserv.arn
  }

  user_data = base64encode(templatefile("${path.module}/user_data_tileserv.sh.tpl", {
    database_url        = "postgresql://${var.db_username}:${var.db_password}@${aws_db_instance.replica.address}:${var.db_port}/${var.db_name}?sslmode=require"
    tileserv_port       = var.tileserv_port
    tileserv_nginx_port = var.tileserv_nginx_port
    pg_tileserv_version = var.pg_tileserv_version
  }))

  lifecycle {
    create_before_destroy = true
  }

  tags = { Name = "${var.app_name}-tileserv-lt" }
}

# ── Auto Scaling Group ────────────────────────────────────────────────────────

resource "aws_autoscaling_group" "tileserv" {
  name                = "${var.app_name}-tileserv-asg"
  min_size            = var.tileserv_asg_min_size
  max_size            = var.tileserv_asg_max_size
  desired_capacity    = var.tileserv_asg_desired_capacity
  vpc_zone_identifier = aws_subnet.private_app[*].id

  launch_template {
    id      = aws_launch_template.tileserv.id
    version = "$Latest"
  }

  target_group_arns         = [aws_lb_target_group.tileserv.arn]
  health_check_type         = "ELB"
  health_check_grace_period = 90

  instance_refresh {
    strategy = "Rolling"
    preferences {
      min_healthy_percentage = 50
    }
  }

  dynamic "tag" {
    for_each = {
      Name = "${var.app_name}-tileserv"
      Env  = var.environment
    }
    content {
      key                 = tag.key
      value               = tag.value
      propagate_at_launch = true
    }
  }
}
