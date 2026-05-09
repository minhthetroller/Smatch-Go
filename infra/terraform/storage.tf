# ── Storage Account ───────────────────────────────────────────────────────────

resource "azurerm_storage_account" "main" {
  name                     = "${var.app_name}str${random_id.suffix.hex}"
  resource_group_name      = azurerm_resource_group.main.name
  location                 = azurerm_resource_group.main.location
  account_tier             = "Standard"
  account_replication_type = "LRS"
  min_tls_version          = "TLS1_2"

  tags = { Name = "${var.app_name}-storage" }
}

# ── Blob Containers ───────────────────────────────────────────────────────────

resource "azurerm_storage_container" "profile" {
  name                  = var.storage_container_profile
  storage_account_id    = azurerm_storage_account.main.id
  container_access_type = "private"
}

resource "azurerm_storage_container" "matches" {
  name                  = var.storage_container_matches
  storage_account_id    = azurerm_storage_account.main.id
  container_access_type = "private"
}

resource "azurerm_storage_container" "business_docs" {
  name                  = var.storage_container_business_docs
  storage_account_id    = azurerm_storage_account.main.id
  container_access_type = "private"
}
