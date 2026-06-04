output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer. Use http://<value> to reach the API."
  value       = aws_lb.main.dns_name
}

output "api_url" {
  description = "Public API URL (domain if configured, otherwise ALB DNS over HTTP)"
  value       = var.create_dns ? "https://${local.api_fqdn}" : "http://${aws_lb.main.dns_name}"
}

output "rds_endpoint" {
  description = "RDS PostgreSQL endpoint (host only, no port)"
  value       = aws_db_instance.main.address
}

output "rds_port" {
  value = aws_db_instance.main.port
}

output "database_url" {
  description = "Full PostgreSQL connection string (for migrate.sh and pg_tileserv)"
  value       = "postgresql://${var.db_username}:${var.db_password}@${aws_db_instance.main.address}:${aws_db_instance.main.port}/${var.db_name}?sslmode=disable"
  sensitive   = true
}

output "redis_endpoint" {
  description = "ElastiCache Redis endpoint host"
  value       = aws_elasticache_cluster.main.cache_nodes[0].address
}

output "redis_port" {
  value = aws_elasticache_cluster.main.port
}

output "s3_bucket_profile" {
  value = aws_s3_bucket.profile.bucket
}

output "s3_bucket_matches" {
  value = aws_s3_bucket.matches.bucket
}

output "web_bucket_name" {
  description = "Private S3 bucket that stores the built admin web assets."
  value       = aws_s3_bucket.web.bucket
}

output "web_cloudfront_distribution_id" {
  description = "CloudFront distribution ID for the admin web frontend."
  value       = aws_cloudfront_distribution.web.id
}

output "web_cloudfront_domain_name" {
  description = "CloudFront domain name for the admin web frontend."
  value       = aws_cloudfront_distribution.web.domain_name
}

output "admin_url" {
  description = "Public admin web URL."
  value       = var.admin_domain_name != "" ? "https://${var.admin_domain_name}" : "https://${aws_cloudfront_distribution.web.domain_name}"
}

output "admin_hosted_zone_id" {
  description = "Route53 hosted zone ID for the admin web domain."
  value       = var.create_dns && var.admin_domain_name != "" ? aws_route53_zone.admin_web[0].zone_id : null
}

output "admin_nameservers" {
  description = "Nameservers to delegate at the external registrar for the admin web domain."
  value       = var.create_dns && var.admin_domain_name != "" ? aws_route53_zone.admin_web[0].name_servers : []
}

output "asg_name" {
  value = aws_autoscaling_group.backend.name
}

output "backend_target_group_arn" {
  description = "Target group ARN for backend ALB target health checks."
  value       = aws_lb_target_group.backend.arn
}

output "admin_asg_name" {
  value = aws_autoscaling_group.admin.name
}

output "rds_replica_endpoint" {
  description = "RDS read replica endpoint (host only)"
  value       = aws_db_instance.replica.address
}

output "rds_proxy_endpoint" {
  description = "Existing RDS Proxy endpoint; pg_tileserv now connects directly to the read replica"
  value       = aws_db_proxy.tileserv.endpoint
}

output "tileserv_database_url" {
  description = "DATABASE_URL for pg_tileserv (direct read replica connection)"
  value       = local.tileserv_database_url
  sensitive   = true
}

output "tileserv_ecs_cluster_name" {
  value = aws_ecs_cluster.tileserv.name
}

output "tileserv_ecs_service_name" {
  value = aws_ecs_service.tileserv.name
}

output "backend_log_group" {
  value = aws_cloudwatch_log_group.backend.name
}

output "admin_log_group" {
  value = aws_cloudwatch_log_group.admin.name
}

output "tileserv_log_group" {
  value = aws_cloudwatch_log_group.tileserv.name
}

output "incident_sns_topic_arn" {
  value = aws_sns_topic.incident_alerts.arn
}

output "log_alarm_notifier_function_name" {
  value = aws_lambda_function.log_alarm_notifier.function_name
}

output "backend_cpu_alarm_name" {
  value = aws_cloudwatch_metric_alarm.backend_cpu_high.alarm_name
}

output "admin_cpu_alarm_name" {
  value = aws_cloudwatch_metric_alarm.admin_cpu_high.alarm_name
}

output "infrastructure_dashboard_name" {
  description = "CloudWatch dashboard for RDS, EC2, host, and ECS infrastructure metrics."
  value       = aws_cloudwatch_dashboard.infrastructure.dashboard_name
}
