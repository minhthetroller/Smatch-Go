output "load_balancer_ip" {
  description = "Public IP of the Azure Load Balancer"
  value       = azurerm_public_ip.lb.ip_address
}

output "api_url" {
  description = "Public API URL"
  value       = "http://${azurerm_public_ip.lb.ip_address}"
}

output "postgres_fqdn" {
  description = "PostgreSQL Flexible Server FQDN"
  value       = azurerm_postgresql_flexible_server.main.fqdn
}

output "postgres_port" {
  value = var.db_port
}

output "database_url" {
  description = "Full PostgreSQL connection string"
  value       = "postgresql://${var.db_username}:${var.db_password}@${azurerm_postgresql_flexible_server.main.fqdn}:${var.db_port}/${var.db_name}?sslmode=require"
  sensitive   = true
}

output "redis_hostname" {
  description = "Azure Cache for Redis hostname"
  value       = azurerm_redis_cache.main.hostname
}

output "redis_ssl_port" {
  description = "Azure Cache for Redis SSL port"
  value       = azurerm_redis_cache.main.ssl_port
}

output "storage_account_name" {
  description = "Azure Storage Account name"
  value       = azurerm_storage_account.main.name
}

output "storage_account_key" {
  description = "Azure Storage Account primary access key"
  value       = azurerm_storage_account.main.primary_access_key
  sensitive   = true
}

output "acr_login_server" {
  description = "Azure Container Registry login server"
  value       = azurerm_container_registry.main.login_server
}

output "key_vault_name" {
  description = "Azure Key Vault name"
  value       = azurerm_key_vault.main.name
}

output "vmss_backend_name" {
  value = azurerm_linux_virtual_machine_scale_set.backend.name
}

output "vmss_admin_name" {
  value = azurerm_linux_virtual_machine_scale_set.admin.name
}

output "vmss_tileserv_name" {
  value = azurerm_linux_virtual_machine_scale_set.tileserv.name
}
