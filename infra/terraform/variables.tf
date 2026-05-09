# ── General ───────────────────────────────────────────────────────────────────

variable "app_name" {
  description = "Name prefix for all Azure resources"
  type        = string
  default     = "smatch"
}

variable "environment" {
  description = "Deployment environment (staging | production)"
  type        = string
  default     = "staging"
}

variable "azure_region" {
  description = "Azure region"
  type        = string
  default     = "southeastasia"
}

# ── Networking ────────────────────────────────────────────────────────────────

variable "vnet_address_space" {
  description = "Address space for the Virtual Network"
  type        = list(string)
  default     = ["10.0.0.0/16"]
}

variable "public_subnet_cidrs" {
  description = "CIDR blocks for public subnets (Application Gateway)"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_app_subnet_cidrs" {
  description = "CIDR blocks for private app subnets (VMSS instances)"
  type        = list(string)
  default     = ["10.0.3.0/24", "10.0.4.0/24"]
}

variable "private_data_subnet_cidrs" {
  description = "CIDR blocks for private data subnets (PostgreSQL, Redis)"
  type        = list(string)
  default     = ["10.0.5.0/24", "10.0.6.0/24"]
}

# ── Compute / VMSS ────────────────────────────────────────────────────────────

variable "vmss_instance_type" {
  description = "Azure VM size for VMSS instances"
  type        = string
  default     = "Standard_D2s_v3"
}

variable "vmss_admin_username" {
  description = "Admin username for VMSS Linux instances"
  type        = string
  default     = "smatch"
}

variable "vmss_ssh_public_key" {
  description = "SSH public key for VMSS admin user"
  type        = string
  sensitive   = true
}

variable "backend_asg_min_size" {
  description = "Minimum number of instances in backend VMSS"
  type        = number
  default     = 1
}

variable "backend_asg_max_size" {
  description = "Maximum number of instances in backend VMSS"
  type        = number
  default     = 3
}

variable "backend_asg_desired_capacity" {
  description = "Desired number of instances in backend VMSS"
  type        = number
  default     = 2
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
  description = "PostgreSQL admin username"
  type        = string
  default     = "psqladmin"
}

variable "db_password" {
  description = "PostgreSQL admin password (minimum 8 characters)"
  type        = string
  sensitive   = true
}

variable "db_port" {
  description = "PostgreSQL port"
  type        = number
  default     = 5432
}

# ── Redis ─────────────────────────────────────────────────────────────────────

variable "redis_sku" {
  description = "Azure Cache for Redis SKU (Basic | Standard | Premium)"
  type        = string
  default     = "Basic"
}

variable "redis_family" {
  description = "Azure Cache for Redis family (C = Basic/Standard, P = Premium)"
  type        = string
  default     = "C"
}

variable "redis_capacity" {
  description = "Azure Cache for Redis capacity (0-6 depending on SKU)"
  type        = number
  default     = 0
}

# ── Storage (Blob) ────────────────────────────────────────────────────────────

variable "storage_container_profile" {
  description = "Blob container name for profile photos"
  type        = string
  default     = "smatch-profiles"
}

variable "storage_container_matches" {
  description = "Blob container name for match media"
  type        = string
  default     = "smatch-matches"
}

variable "storage_container_business_docs" {
  description = "Blob container name for court owner business documents"
  type        = string
  default     = "smatch-business-docs"
}

# ── DNS / TLS ─────────────────────────────────────────────────────────────────

variable "domain_name" {
  description = "Root domain name (e.g. example.com). Leave empty to skip DNS/TLS setup."
  type        = string
  default     = ""
}

variable "api_subdomain" {
  description = "Subdomain for the API (e.g. api → api.example.com)"
  type        = string
  default     = "api"
}

variable "create_dns" {
  description = "Set to true to create DNS records and TLS certificates"
  type        = bool
  default     = false
}

# ── Application config (injected via cloud-init) ──────────────────────────────

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

# ── Admin backend VMSS ────────────────────────────────────────────────────────

variable "admin_domain_name" {
  description = "Full domain name for the admin backend (e.g. admin-smb.online)"
  type        = string
  default     = ""
}

variable "admin_asg_min_size" {
  description = "Minimum instances in admin VMSS"
  type        = number
  default     = 1
}

variable "admin_asg_max_size" {
  description = "Maximum instances in admin VMSS"
  type        = number
  default     = 2
}

variable "admin_asg_desired_capacity" {
  description = "Desired instances in admin VMSS"
  type        = number
  default     = 1
}

# ── pg_tileserv VMSS ──────────────────────────────────────────────────────────

variable "tileserv_asg_min_size" {
  description = "Minimum instances in tileserv VMSS"
  type        = number
  default     = 1
}

variable "tileserv_asg_max_size" {
  description = "Maximum instances in tileserv VMSS"
  type        = number
  default     = 3
}

variable "tileserv_asg_desired_capacity" {
  description = "Desired instances in tileserv VMSS"
  type        = number
  default     = 2
}

variable "tileserv_port" {
  description = "Port pg_tileserv listens on"
  type        = number
  default     = 7800
}

variable "tileserv_nginx_port" {
  description = "Port nginx listens on for tileserv"
  type        = number
  default     = 80
}

variable "pg_tileserv_version" {
  description = "pg_tileserv release version to download from GitHub"
  type        = string
  default     = "1.0.11"
}

# ── Container Registry ────────────────────────────────────────────────────────

variable "acr_name" {
  description = "Azure Container Registry name (globally unique, alphanumeric only)"
  type        = string
  default     = ""
}
