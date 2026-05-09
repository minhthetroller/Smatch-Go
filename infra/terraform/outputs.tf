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

output "asg_name" {
  value = aws_autoscaling_group.backend.name
}

output "admin_asg_name" {
  value = aws_autoscaling_group.admin.name
}

output "tileserv_asg_name" {
  value = aws_autoscaling_group.tileserv.name
}

output "rds_replica_endpoint" {
  description = "RDS read replica endpoint (host only)"
  value       = aws_db_instance.replica.address
}

output "rds_proxy_endpoint" {
  description = "RDS Proxy endpoint used by pg_tileserv"
  value       = aws_db_proxy.tileserv.endpoint
}

output "tileserv_database_url" {
  description = "DATABASE_URL for pg_tileserv (via RDS Proxy)"
  value       = "postgresql://${var.db_username}:${var.db_password}@${aws_db_proxy.tileserv.endpoint}:${var.db_port}/${var.db_name}?sslmode=require"
  sensitive   = true
}
