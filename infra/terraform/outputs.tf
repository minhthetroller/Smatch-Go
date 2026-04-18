output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer. Use http://<value> to reach the API."
  value       = aws_lb.main.dns_name
}

output "api_url" {
  description = "Public API URL (domain if configured, otherwise ALB DNS over HTTP)"
  value       = var.create_dns ? "https://${local.fqdn}" : "http://${aws_lb.main.dns_name}"
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
