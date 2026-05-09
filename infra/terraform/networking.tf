# ── Virtual Network ───────────────────────────────────────────────────────────

resource "azurerm_virtual_network" "main" {
  name                = "${var.app_name}-vnet"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  address_space       = var.vnet_address_space
}

# ── Subnets ───────────────────────────────────────────────────────────────────

resource "azurerm_subnet" "public" {
  count                = length(var.public_subnet_cidrs)
  name                 = "${var.app_name}-snet-public-${count.index + 1}"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [var.public_subnet_cidrs[count.index]]
}

resource "azurerm_subnet" "private_app" {
  count                = length(var.private_app_subnet_cidrs)
  name                 = "${var.app_name}-snet-app-${count.index + 1}"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [var.private_app_subnet_cidrs[count.index]]
}

# Delegate private data subnet to PostgreSQL Flexible Server
resource "azurerm_subnet" "private_data" {
  count                = length(var.private_data_subnet_cidrs)
  name                 = "${var.app_name}-snet-data-${count.index + 1}"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [var.private_data_subnet_cidrs[count.index]]

  delegation {
    name = "postgresql-delegation"
    service_delegation {
      name = "Microsoft.DBforPostgreSQL/flexibleServers"
      actions = [
        "Microsoft.Network/virtualNetworks/subnets/join/action",
      ]
    }
  }
}

# ── Public IP for NAT Gateway ─────────────────────────────────────────────────

resource "azurerm_public_ip" "nat" {
  name                = "${var.app_name}-pip-nat"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  allocation_method   = "Static"
  sku                 = "Standard"
}

resource "azurerm_nat_gateway" "main" {
  name                    = "${var.app_name}-nat"
  resource_group_name     = azurerm_resource_group.main.name
  location                = azurerm_resource_group.main.location
  sku_name                = "Standard"
  idle_timeout_in_minutes = 10
}

resource "azurerm_nat_gateway_public_ip_association" "main" {
  nat_gateway_id       = azurerm_nat_gateway.main.id
  public_ip_address_id = azurerm_public_ip.nat.id
}

# Associate NAT Gateway with app subnets for outbound connectivity
resource "azurerm_subnet_nat_gateway_association" "private_app" {
  count          = length(azurerm_subnet.private_app)
  subnet_id      = azurerm_subnet.private_app[count.index].id
  nat_gateway_id = azurerm_nat_gateway.main.id
}

# ── Network Security Groups ───────────────────────────────────────────────────

resource "azurerm_network_security_group" "app_gateway" {
  name                = "${var.app_name}-nsg-agw"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location

  security_rule {
    name                       = "AllowHTTP"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "80"
    source_address_prefix      = "Internet"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "AllowHTTPS"
    priority                   = 110
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "443"
    source_address_prefix      = "Internet"
    destination_address_prefix = "*"
  }
}

resource "azurerm_subnet_network_security_group_association" "public" {
  for_each                  = { for i, s in azurerm_subnet.public : i => s.id }
  subnet_id                 = each.value
  network_security_group_id = azurerm_network_security_group.app_gateway.id
}

resource "azurerm_network_security_group" "backend" {
  name                = "${var.app_name}-nsg-backend"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location

  security_rule {
    name                       = "AllowAzureLB"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = tostring(var.backend_port)
    source_address_prefix      = "AzureLoadBalancer"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "AllowInternetInbound"
    priority                   = 110
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = tostring(var.backend_port)
    source_address_prefix      = "Internet"
    destination_address_prefix = "*"
  }

  security_rule {
    name                       = "AllowInternetHTTPS"
    priority                   = 120
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "443"
    source_address_prefix      = "Internet"
    destination_address_prefix = "*"
  }
}

resource "azurerm_subnet_network_security_group_association" "private_app" {
  for_each                  = { for i, s in azurerm_subnet.private_app : i => s.id }
  subnet_id                 = each.value
  network_security_group_id = azurerm_network_security_group.backend.id
}

resource "azurerm_network_security_group" "postgres" {
  name                = "${var.app_name}-nsg-postgres"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location

  security_rule {
    name                       = "AllowFromApp"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = tostring(var.db_port)
    source_address_prefixes    = var.private_app_subnet_cidrs
    destination_address_prefix = "*"
  }
}

resource "azurerm_subnet_network_security_group_association" "private_data" {
  for_each                  = { for i, s in azurerm_subnet.private_data : i => s.id }
  subnet_id                 = each.value
  network_security_group_id = azurerm_network_security_group.postgres.id
}
