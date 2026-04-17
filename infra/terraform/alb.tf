# ── ALB ───────────────────────────────────────────────────────────────────────

resource "aws_lb" "main" {
  name               = "${var.app_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  tags = { Name = "${var.app_name}-alb" }
}

# ── Target Group ──────────────────────────────────────────────────────────────

resource "aws_lb_target_group" "backend" {
  name        = "${var.app_name}-tg-backend"
  port        = var.backend_port
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "instance"

  health_check {
    enabled             = true
    path                = "/health"
    protocol            = "HTTP"
    port                = tostring(var.backend_port)
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 15
    matcher             = "200"
  }

  tags = { Name = "${var.app_name}-tg-backend" }
}

# ── ACM Certificate ───────────────────────────────────────────────────────────
# Only created when create_dns = true and domain_name is set.

locals {
  fqdn = var.domain_name != "" ? (
    var.api_subdomain != "" ? "${var.api_subdomain}.${var.domain_name}" : var.domain_name
  ) : ""
}

resource "aws_acm_certificate" "api" {
  count             = var.create_dns ? 1 : 0
  domain_name       = local.fqdn
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = { Name = "${var.app_name}-cert" }
}

# ── Route53 ───────────────────────────────────────────────────────────────────

data "aws_route53_zone" "main" {
  count        = var.create_dns ? 1 : 0
  name         = var.domain_name
  private_zone = false
}

# DNS validation record for ACM
resource "aws_route53_record" "cert_validation" {
  count   = var.create_dns ? 1 : 0
  zone_id = data.aws_route53_zone.main[0].zone_id
  name    = tolist(aws_acm_certificate.api[0].domain_validation_options)[0].resource_record_name
  type    = tolist(aws_acm_certificate.api[0].domain_validation_options)[0].resource_record_type
  records = [tolist(aws_acm_certificate.api[0].domain_validation_options)[0].resource_record_value]
  ttl     = 60
}

resource "aws_acm_certificate_validation" "api" {
  count                   = var.create_dns ? 1 : 0
  certificate_arn         = aws_acm_certificate.api[0].arn
  validation_record_fqdns = [aws_route53_record.cert_validation[0].fqdn]
}

# A record: api.yourdomain.com → ALB
resource "aws_route53_record" "api" {
  count   = var.create_dns ? 1 : 0
  zone_id = data.aws_route53_zone.main[0].zone_id
  name    = local.fqdn
  type    = "A"

  alias {
    name                   = aws_lb.main.dns_name
    zone_id                = aws_lb.main.zone_id
    evaluate_target_health = true
  }
}

# ── HTTP Listener: redirect to HTTPS (when domain is set) ────────────────────

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = var.create_dns ? "redirect" : "forward"

    dynamic "redirect" {
      for_each = var.create_dns ? [1] : []
      content {
        port        = "443"
        protocol    = "HTTPS"
        status_code = "HTTP_301"
      }
    }

    dynamic "forward" {
      for_each = var.create_dns ? [] : [1]
      content {
        target_group {
          arn = aws_lb_target_group.backend.arn
        }
      }
    }
  }
}

# ── HTTPS Listener (only when domain + cert are configured) ──────────────────

resource "aws_lb_listener" "https" {
  count             = var.create_dns ? 1 : 0
  load_balancer_arn = aws_lb.main.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate_validation.api[0].certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.backend.arn
  }
}
