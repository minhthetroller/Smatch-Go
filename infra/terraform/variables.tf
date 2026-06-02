# ── General ───────────────────────────────────────────────────────────────────

variable "app_name" {
  description = "Name prefix for all AWS resources"
  type        = string
  default     = "smatch"
}

variable "environment" {
  description = "Deployment environment (staging | production)"
  type        = string
  default     = "staging"
}

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "ap-southeast-1"
}

variable "aws_profile" {
  description = "AWS CLI profile from ~/.aws/credentials"
  type        = string
  default     = "default"
}

variable "aws_endpoint" {
  description = "Override AWS endpoint URL (set to LocalStack URL for local dev; leave empty for real AWS)"
  type        = string
  default     = ""
}

# ── Networking ────────────────────────────────────────────────────────────────

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "List of availability zones to deploy into"
  type        = list(string)
  default     = ["ap-southeast-1a", "ap-southeast-1b"]
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for public subnets (ALB)"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_app_subnet_cidrs" {
  description = "CIDR blocks for private app subnets (ASG / EC2 instances / ECS tasks)"
  type        = list(string)
  default     = ["10.0.3.0/24", "10.0.4.0/24"]
}

variable "private_subnet_cidrs" {
  description = "CIDR blocks for private data subnets (RDS, ElastiCache)"
  type        = list(string)
  default     = ["10.0.5.0/24", "10.0.6.0/24"]
}

# ── EC2 / ASG ─────────────────────────────────────────────────────────────────

variable "ami_id" {
  description = "AMI ID for EC2 instances in the ASG"
  type        = string
}

variable "instance_type" {
  description = "EC2 instance type for backend instances"
  type        = string
  default     = "t3.small"
}

variable "admin_ami_id" {
  description = "Optional AMI ID for admin EC2 instances. Leave empty to reuse ami_id."
  type        = string
  default     = ""
}

variable "admin_instance_type" {
  description = "Optional EC2 instance type for admin instances. Leave empty to reuse instance_type."
  type        = string
  default     = ""
}

variable "asg_min_size" {
  description = "Minimum number of EC2 instances in the ASG"
  type        = number
  default     = 1
}

variable "asg_max_size" {
  description = "Maximum number of EC2 instances in the ASG"
  type        = number
  default     = 3
}

variable "asg_desired_capacity" {
  description = "Desired number of EC2 instances in the ASG"
  type        = number
  default     = 2
}

variable "asg_cpu_target_percent" {
  description = "Target average CPU utilization percentage for ASG target tracking"
  type        = number
  default     = 60
}

variable "asg_request_count_target_per_minute" {
  description = "Target ALB requests per target per minute for ASG target tracking"
  type        = number
  default     = 100
}

variable "ecr_repo_url" {
  description = "Full ECR repository URL for the backend Docker image (e.g. 123456789012.dkr.ecr.ap-southeast-1.amazonaws.com/smatch-backend)"
  type        = string
  default     = ""
}

variable "backend_port" {
  description = "Port the backend API listens on"
  type        = number
  default     = 3000
}

# ── Database ──────────────────────────────────────────────────────────────────

variable "db_name" {
  description = "PostgreSQL database name"
  type        = string
  default     = "smatch"
}

variable "db_username" {
  description = "PostgreSQL master username"
  type        = string
  default     = "postgres"
}

variable "db_password" {
  description = "PostgreSQL master password (minimum 12 characters)"
  type        = string
  sensitive   = true
}

variable "db_port" {
  description = "PostgreSQL port"
  type        = number
  default     = 5432
}

# ── Redis ─────────────────────────────────────────────────────────────────────

variable "redis_port" {
  description = "ElastiCache Redis port"
  type        = number
  default     = 6379
}

variable "redis_password" {
  description = "AUTH token for ElastiCache Redis (leave empty to disable auth, not recommended for production)"
  type        = string
  sensitive   = true
  default     = ""
}

# ── S3 ────────────────────────────────────────────────────────────────────────

variable "s3_bucket_profile" {
  description = "S3 bucket name for profile photos"
  type        = string
  default     = "smatch-profiles"
}

variable "s3_bucket_matches" {
  description = "S3 bucket name for match media"
  type        = string
  default     = "smatch-matches"
}

variable "s3_bucket_business_docs" {
  description = "S3 bucket name for court owner business documents"
  type        = string
  default     = "smatch-business-docs"
}

variable "web_bucket_name" {
  description = "S3 bucket name for the admin web static assets. Leave empty to derive a unique name."
  type        = string
  default     = ""
}

variable "web_bucket_force_destroy" {
  description = "Whether Terraform can delete the admin web bucket even when it contains objects."
  type        = bool
  default     = true
}

# ── DNS / TLS ─────────────────────────────────────────────────────────────────

variable "domain_name" {
  description = "Root domain name (e.g. example.com). Leave empty to skip DNS/TLS setup."
  type        = string
  default     = ""
}

variable "api_subdomain" {
  description = "Subdomain for the API (e.g. api → api.example.com). Leave empty to use the domain root."
  type        = string
  default     = "api"
}

variable "create_dns" {
  description = "Set to true to create Route53 DNS records and an ACM certificate"
  type        = bool
  default     = false
}

# ── Application config (injected via user-data) ───────────────────────────────

variable "firebase_credentials_file" {
  description = "Path inside the container to the Firebase service-account JSON file"
  type        = string
  default     = "/app/smatch-badminton-firebase-adminsdk-fbsvc-fb65abab30.json"
}

variable "zalopay_app_id" {
  description = "ZaloPay application ID"
  type        = string
  default     = ""
}

variable "zalopay_key1" {
  type      = string
  sensitive = true
  default   = ""
}

variable "zalopay_key2" {
  type      = string
  sensitive = true
  default   = ""
}

variable "zalopay_endpoint" {
  type    = string
  default = "https://sb-openapi.zalopay.vn"
}

variable "zalopay_callback_url" {
  type    = string
  default = ""
}

variable "tile_server_url" {
  type    = string
  default = "http://localhost:7800"
}

variable "admin_secret" {
  type      = string
  sensitive = true
  default   = ""
}

variable "rate_limit_trusted_ips" {
  description = "Comma-separated list of trusted IP CIDRs that bypass rate limiting"
  type        = string
  default     = ""
}

# ── Admin backend ASG ─────────────────────────────────────────────────────────

variable "admin_domain_name" {
  description = "Full domain name for the admin web frontend (e.g. admin-sb.online). CloudFront proxies /api/* to the admin backend."
  type        = string
  default     = ""
}

variable "admin_asg_min_size" {
  description = "Minimum instances in admin ASG"
  type        = number
  default     = 1
}

variable "admin_asg_max_size" {
  description = "Maximum instances in admin ASG"
  type        = number
  default     = 2
}

variable "admin_asg_desired_capacity" {
  description = "Desired instances in admin ASG"
  type        = number
  default     = 1
}

# ── pg_tileserv ECS/Fargate ──────────────────────────────────────────────────

variable "tileserv_desired_count" {
  description = "Desired number of pg_tileserv Fargate tasks"
  type        = number
  default     = 2
}

variable "tileserv_task_cpu" {
  description = "CPU units for the pg_tileserv Fargate task"
  type        = number
  default     = 512
}

variable "tileserv_task_memory" {
  description = "Memory in MiB for the pg_tileserv Fargate task"
  type        = number
  default     = 1024
}

variable "tileserv_image_url" {
  description = "Container image for pg_tileserv"
  type        = string
  default     = "pramsey/pg_tileserv:latest"
}

variable "tileserv_nginx_image_url" {
  description = "Container image for the pg_tileserv nginx rewrite proxy"
  type        = string
  default     = "nginx:1.27-alpine"
}

variable "tileserv_port" {
  description = "Port pg_tileserv binary listens on"
  type        = number
  default     = 7800
}

variable "tileserv_nginx_port" {
  description = "Port nginx listens on for tileserv (the port ALB forwards to)"
  type        = number
  default     = 80
}

# ── Observability / incident email ───────────────────────────────────────────

variable "log_retention_days" {
  description = "CloudWatch Logs retention period in days"
  type        = number
  default     = 14
}

variable "incident_email" {
  description = "Email address subscribed to incident SNS notifications"
  type        = string
  default     = "nguyentuanminh1105@gmail.com"
}

variable "cpu_alarm_threshold" {
  description = "Average ASG CPU percentage that triggers incident notifications"
  type        = number
  default     = 70
}

variable "cpu_alarm_period_seconds" {
  description = "CloudWatch CPU alarm period in seconds"
  type        = number
  default     = 60
}

variable "cpu_alarm_evaluation_periods" {
  description = "Number of periods evaluated by the CPU alarm"
  type        = number
  default     = 2
}

variable "lambda_log_lookback_minutes" {
  description = "Minutes of logs queried by the incident Lambda"
  type        = number
  default     = 15
}

variable "fis_cpu_stress_duration_seconds" {
  description = "Duration in seconds used by AWS FIS CPU stress experiments"
  type        = number
  default     = 900
}

variable "fis_cpu_stress_percent" {
  description = "CPU stress percentage used by AWS FIS experiments"
  type        = number
  default     = 85
}

variable "fis_alb_load_duration_seconds" {
  description = "Duration in seconds used by AWS FIS ALB HTTP load experiments"
  type        = number
  default     = 900
}

variable "fis_alb_load_concurrency" {
  description = "Number of parallel curl workers used by AWS FIS ALB HTTP load experiments"
  type        = number
  default     = 25
}
