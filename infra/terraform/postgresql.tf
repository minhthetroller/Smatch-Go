# ── Azure Database for PostgreSQL Flexible Server ─────────────────────────────

resource "azurerm_postgresql_flexible_server" "main" {
  name                   = "${var.app_name}psql${random_id.suffix.hex}"
  resource_group_name    = azurerm_resource_group.main.name
  location               = azurerm_resource_group.main.location
  version                = "15"
  delegated_subnet_id    = azurerm_subnet.private_data[0].id
  private_dns_zone_id    = azurerm_private_dns_zone.postgres.id
  administrator_login    = var.db_username
  administrator_password = var.db_password
  storage_mb             = 32768
  storage_tier           = "P4"

  sku_name   = "B_Standard_B1ms"
  zone       = "1"

  public_network_access_enabled = false

  tags = { Name = "${var.app_name}-postgres" }
}

resource "azurerm_postgresql_flexible_server_database" "main" {
  name      = var.db_name
  server_id = azurerm_postgresql_flexible_server.main.id
}

resource "azurerm_postgresql_flexible_server_configuration" "postgis" {
  name      = "azure.extensions"
  server_id = azurerm_postgresql_flexible_server.main.id
  value     = "POSTGIS"
}

# ── Private DNS Zone for PostgreSQL ───────────────────────────────────────────

resource "azurerm_private_dns_zone" "postgres" {
  name                = "${var.app_name}.private.postgres.database.azure.com"
  resource_group_name = azurerm_resource_group.main.name
}

resource "azurerm_private_dns_zone_virtual_network_link" "postgres" {
  name                  = "${var.app_name}-postgres-dns-link"
  resource_group_name   = azurerm_resource_group.main.name
  private_dns_zone_name = azurerm_private_dns_zone.postgres.name
  virtual_network_id    = azurerm_virtual_network.main.id
}
