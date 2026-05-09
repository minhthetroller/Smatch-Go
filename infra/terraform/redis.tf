# ── Azure Cache for Redis ─────────────────────────────────────────────────────

resource "azurerm_redis_cache" "main" {
  name                 = "${var.app_name}redis${random_id.suffix.hex}"
  resource_group_name  = azurerm_resource_group.main.name
  location             = azurerm_resource_group.main.location
  capacity             = var.redis_capacity
  family               = var.redis_family
  sku_name             = var.redis_sku
  non_ssl_port_enabled  = false
  minimum_tls_version  = "1.2"

  tags = { Name = "${var.app_name}-redis" }
}
