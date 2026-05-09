# ── ALB ───────────────────────────────────────────────────────────────────────

resource "aws_lb" "main" {
  name               = "${var.app_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  tags = { Name = "${var.app_name}-alb" }
}

# ── Target Groups ─────────────────────────────────────────────────────────────

# User backend
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

# Admin backend
resource "aws_lb_target_group" "admin" {
  name        = "${var.app_name}-tg-admin"
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

  tags = { Name = "${var.app_name}-tg-admin" }
}

# pg_tileserv (traffic hits nginx, nginx proxies to pg_tileserv on localhost:7800)
resource "aws_lb_target_group" "tileserv" {
  name        = "${var.app_name}-tg-tileserv"
  port        = var.tileserv_nginx_port
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "instance"

  health_check {
    enabled             = true
    path                = "/public.courts/0/0/0.pbf"
    protocol            = "HTTP"
    port                = tostring(var.tileserv_nginx_port)
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    matcher             = "200,204"
  }

  tags = { Name = "${var.app_name}-tg-tileserv" }
}

# ── ACM Certificates ──────────────────────────────────────────────────────────
# Only created when create_dns = true.

locals {
  api_fqdn = var.domain_name != "" ? (
    var.api_subdomain != "" ? "${var.api_subdomain}.${var.domain_name}" : var.domain_name
  ) : ""
}

# Certificate for user API domain (api-smatch.sbs)
resource "aws_acm_certificate" "api" {
  count             = var.create_dns ? 1 : 0
  domain_name       = local.api_fqdn
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = { Name = "${var.app_name}-cert-api" }
}

# Certificate for admin domain (admin-smb.online)
resource "aws_acm_certificate" "admin" {
  count             = var.create_dns && var.admin_domain_name != "" ? 1 : 0
  domain_name       = var.admin_domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = { Name = "${var.app_name}-cert-admin" }
}

# ── Route53 ───────────────────────────────────────────────────────────────────

data "aws_route53_zone" "api" {
  count        = var.create_dns ? 1 : 0
  name         = var.domain_name
  private_zone = false
}

data "aws_route53_zone" "admin" {
  count        = var.create_dns && var.admin_domain_name != "" ? 1 : 0
  name         = var.admin_domain_name
  private_zone = false
}

# DNS validation record for API cert
resource "aws_route53_record" "api_cert_validation" {
  count   = var.create_dns ? 1 : 0
  zone_id = data.aws_route53_zone.api[0].zone_id
  name    = tolist(aws_acm_certificate.api[0].domain_validation_options)[0].resource_record_name
  type    = tolist(aws_acm_certificate.api[0].domain_validation_options)[0].resource_record_type
  records = [tolist(aws_acm_certificate.api[0].domain_validation_options)[0].resource_record_value]
  ttl     = 60
}

resource "aws_acm_certificate_validation" "api" {
  count                   = var.create_dns ? 1 : 0
  certificate_arn         = aws_acm_certificate.api[0].arn
  validation_record_fqdns = [aws_route53_record.api_cert_validation[0].fqdn]
}

# DNS validation record for admin cert
resource "aws_route53_record" "admin_cert_validation" {
  count   = var.create_dns && var.admin_domain_name != "" ? 1 : 0
  zone_id = data.aws_route53_zone.admin[0].zone_id
  name    = tolist(aws_acm_certificate.admin[0].domain_validation_options)[0].resource_record_name
  type    = tolist(aws_acm_certificate.admin[0].domain_validation_options)[0].resource_record_type
  records = [tolist(aws_acm_certificate.admin[0].domain_validation_options)[0].resource_record_value]
  ttl     = 60
}

resource "aws_acm_certificate_validation" "admin" {
  count                   = var.create_dns && var.admin_domain_name != "" ? 1 : 0
  certificate_arn         = aws_acm_certificate.admin[0].arn
  validation_record_fqdns = [aws_route53_record.admin_cert_validation[0].fqdn]
}

# A records — both domains point to the same ALB
resource "aws_route53_record" "api" {
  count   = var.create_dns ? 1 : 0
  zone_id = data.aws_route53_zone.api[0].zone_id
  name    = local.api_fqdn
  type    = "A"

  alias {
    name                   = aws_lb.main.dns_name
    zone_id                = aws_lb.main.zone_id
    evaluate_target_health = true
  }
}

resource "aws_route53_record" "admin" {
  count   = var.create_dns && var.admin_domain_name != "" ? 1 : 0
  zone_id = data.aws_route53_zone.admin[0].zone_id
  name    = var.admin_domain_name
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

# Attach admin certificate as an additional cert on the HTTPS listener
resource "aws_lb_listener_certificate" "admin" {
  count           = var.create_dns && var.admin_domain_name != "" ? 1 : 0
  listener_arn    = aws_lb_listener.https[0].arn
  certificate_arn = aws_acm_certificate_validation.admin[0].certificate_arn
}

# ── Listener Rules ────────────────────────────────────────────────────────────
# Priority 10: api-smatch.sbs + /api/map-tiles/* → pg_tileserv
resource "aws_lb_listener_rule" "tileserv" {
  count        = var.create_dns ? 1 : 0
  listener_arn = aws_lb_listener.https[0].arn
  priority     = 10

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.tileserv.arn
  }

  condition {
    host_header { values = [local.api_fqdn] }
  }

  condition {
    path_pattern { values = ["/api/map-tiles/*"] }
  }
}

# Priority 20: api-smatch.sbs → user backend
resource "aws_lb_listener_rule" "api" {
  count        = var.create_dns ? 1 : 0
  listener_arn = aws_lb_listener.https[0].arn
  priority     = 20

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.backend.arn
  }

  condition {
    host_header { values = [local.api_fqdn] }
  }
}

# Priority 30: admin-smb.online → admin backend
resource "aws_lb_listener_rule" "admin" {
  count        = var.create_dns && var.admin_domain_name != "" ? 1 : 0
  listener_arn = aws_lb_listener.https[0].arn
  priority     = 30

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.admin.arn
  }

  condition {
    host_header { values = [var.admin_domain_name] }
  }
}
