# ── Azure Managed Redis ────────────────────────────────────────────────────────

resource "azurerm_managed_redis" "main" {
  name                = "${var.app_name}redis${random_id.suffix.hex}"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku_name            = var.managed_redis_sku

  default_database {
    client_protocol                    = "Encrypted"
    clustering_policy                  = "EnterpriseCluster"
    eviction_policy                    = "VolatileLRU"
    access_keys_authentication_enabled = true
  }

  tags = { Name = "${var.app_name}-redis" }
}
