# ── Admin web: private S3 + CloudFront ───────────────────────────────────────

data "aws_caller_identity" "current" {}

data "aws_cloudfront_cache_policy" "caching_optimized" {
  name = "Managed-CachingOptimized"
}

data "aws_cloudfront_cache_policy" "caching_disabled" {
  name = "Managed-CachingDisabled"
}

data "aws_cloudfront_origin_request_policy" "all_viewer" {
  name = "Managed-AllViewer"
}

locals {
  admin_web_custom_domain_enabled = var.create_dns && var.admin_domain_name != ""
  web_bucket_name = var.web_bucket_name != "" ? var.web_bucket_name : (
    "${var.app_name}-${var.environment}-admin-web-${data.aws_caller_identity.current.account_id}"
  )
  web_s3_origin_id  = "${var.app_name}-${var.environment}-admin-web-s3"
  web_api_origin_id = "${var.app_name}-${var.environment}-admin-api-alb"
}

resource "aws_s3_bucket" "web" {
  bucket        = local.web_bucket_name
  force_destroy = var.web_bucket_force_destroy

  tags = { Name = "${var.app_name}-admin-web" }
}

resource "aws_s3_bucket_public_access_block" "web" {
  bucket = aws_s3_bucket.web.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_ownership_controls" "web" {
  bucket = aws_s3_bucket.web.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "web" {
  bucket = aws_s3_bucket.web.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_route53_zone" "admin_web" {
  count = local.admin_web_custom_domain_enabled ? 1 : 0
  name  = var.admin_domain_name

  tags = { Name = "${var.app_name}-admin-web-zone" }
}

resource "aws_acm_certificate" "admin_web" {
  provider          = aws.us_east_1
  count             = local.admin_web_custom_domain_enabled ? 1 : 0
  domain_name       = var.admin_domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = { Name = "${var.app_name}-cert-admin-web" }
}

resource "aws_route53_record" "admin_web_cert_validation" {
  count   = local.admin_web_custom_domain_enabled ? 1 : 0
  zone_id = aws_route53_zone.admin_web[0].zone_id
  name    = tolist(aws_acm_certificate.admin_web[0].domain_validation_options)[0].resource_record_name
  type    = tolist(aws_acm_certificate.admin_web[0].domain_validation_options)[0].resource_record_type
  records = [tolist(aws_acm_certificate.admin_web[0].domain_validation_options)[0].resource_record_value]
  ttl     = 60
}

resource "aws_acm_certificate_validation" "admin_web" {
  provider                = aws.us_east_1
  count                   = local.admin_web_custom_domain_enabled ? 1 : 0
  certificate_arn         = aws_acm_certificate.admin_web[0].arn
  validation_record_fqdns = [aws_route53_record.admin_web_cert_validation[0].fqdn]
}

resource "aws_cloudfront_origin_access_control" "web" {
  name                              = "${var.app_name}-${var.environment}-admin-web-oac"
  description                       = "Allow CloudFront to read the private admin web S3 bucket."
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_response_headers_policy" "web_security" {
  name = "${var.app_name}-${var.environment}-admin-web-security"

  security_headers_config {
    content_type_options {
      override = true
    }

    frame_options {
      frame_option = "DENY"
      override     = true
    }

    referrer_policy {
      referrer_policy = "strict-origin-when-cross-origin"
      override        = true
    }

    strict_transport_security {
      access_control_max_age_sec = 31536000
      include_subdomains         = true
      override                   = true
      preload                    = false
    }

    xss_protection {
      mode_block = true
      override   = true
      protection = true
    }
  }
}

resource "aws_cloudfront_function" "web_spa_rewrite" {
  name    = "${var.app_name}-${var.environment}-admin-web-spa-rewrite"
  runtime = "cloudfront-js-2.0"
  publish = true

  code = <<-JS
function handler(event) {
  var request = event.request;
  var uri = request.uri || "/";

  if (uri.indexOf("/api/") === 0) {
    return request;
  }

  if (uri === "/" || uri.indexOf(".") === -1) {
    request.uri = "/index.html";
  }

  return request;
}
JS
}

resource "aws_cloudfront_distribution" "web" {
  enabled             = true
  is_ipv6_enabled     = true
  comment             = "${var.app_name} ${var.environment} admin web"
  default_root_object = "index.html"
  aliases             = local.admin_web_custom_domain_enabled ? [var.admin_domain_name] : []

  origin {
    domain_name              = aws_s3_bucket.web.bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.web.id
    origin_id                = local.web_s3_origin_id
  }

  origin {
    domain_name = aws_lb.main.dns_name
    origin_id   = local.web_api_origin_id

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "http-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    allowed_methods            = ["GET", "HEAD", "OPTIONS"]
    cached_methods             = ["GET", "HEAD", "OPTIONS"]
    cache_policy_id            = data.aws_cloudfront_cache_policy.caching_optimized.id
    compress                   = true
    response_headers_policy_id = aws_cloudfront_response_headers_policy.web_security.id
    target_origin_id           = local.web_s3_origin_id
    viewer_protocol_policy     = "redirect-to-https"

    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.web_spa_rewrite.arn
    }
  }

  ordered_cache_behavior {
    allowed_methods            = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cache_policy_id            = data.aws_cloudfront_cache_policy.caching_disabled.id
    cached_methods             = ["GET", "HEAD", "OPTIONS"]
    compress                   = true
    origin_request_policy_id   = data.aws_cloudfront_origin_request_policy.all_viewer.id
    path_pattern               = "/api/*"
    response_headers_policy_id = aws_cloudfront_response_headers_policy.web_security.id
    target_origin_id           = local.web_api_origin_id
    viewer_protocol_policy     = "redirect-to-https"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn            = local.admin_web_custom_domain_enabled ? aws_acm_certificate_validation.admin_web[0].certificate_arn : null
    cloudfront_default_certificate = local.admin_web_custom_domain_enabled ? false : true
    minimum_protocol_version       = local.admin_web_custom_domain_enabled ? "TLSv1.2_2021" : null
    ssl_support_method             = local.admin_web_custom_domain_enabled ? "sni-only" : null
  }

  tags = { Name = "${var.app_name}-admin-web-cdn" }
}

resource "aws_s3_bucket_policy" "web" {
  bucket = aws_s3_bucket.web.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowCloudFrontReadOnly"
        Effect    = "Allow"
        Principal = { Service = "cloudfront.amazonaws.com" }
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.web.arn}/*"
        Condition = {
          StringEquals = {
            "AWS:SourceArn" = aws_cloudfront_distribution.web.arn
          }
        }
      }
    ]
  })
}

resource "aws_route53_record" "admin_web" {
  count   = local.admin_web_custom_domain_enabled ? 1 : 0
  zone_id = aws_route53_zone.admin_web[0].zone_id
  name    = var.admin_domain_name
  type    = "A"

  alias {
    evaluate_target_health = false
    name                   = aws_cloudfront_distribution.web.domain_name
    zone_id                = aws_cloudfront_distribution.web.hosted_zone_id
  }
}

resource "aws_route53_record" "admin_web_ipv6" {
  count   = local.admin_web_custom_domain_enabled ? 1 : 0
  zone_id = aws_route53_zone.admin_web[0].zone_id
  name    = var.admin_domain_name
  type    = "AAAA"

  alias {
    evaluate_target_health = false
    name                   = aws_cloudfront_distribution.web.domain_name
    zone_id                = aws_cloudfront_distribution.web.hosted_zone_id
  }
}
