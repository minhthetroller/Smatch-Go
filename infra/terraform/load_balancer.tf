# ── Public IP for Load Balancer ───────────────────────────────────────────────

resource "azurerm_public_ip" "lb" {
  name                = "${var.app_name}-pip-lb"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  allocation_method   = "Static"
  sku                 = "Standard"
}

# ── Load Balancer ─────────────────────────────────────────────────────────────

resource "azurerm_lb" "main" {
  name                = "${var.app_name}-lb"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = "Standard"

  frontend_ip_configuration {
    name                 = "${var.app_name}-fe-lb"
    public_ip_address_id = azurerm_public_ip.lb.id
  }
}

# ── Backend Address Pools ─────────────────────────────────────────────────────

resource "azurerm_lb_backend_address_pool" "backend" {
  name            = "${var.app_name}-bep-backend"
  loadbalancer_id = azurerm_lb.main.id
}

resource "azurerm_lb_backend_address_pool" "admin" {
  name            = "${var.app_name}-bep-admin"
  loadbalancer_id = azurerm_lb.main.id
}

resource "azurerm_lb_backend_address_pool" "tileserv" {
  name            = "${var.app_name}-bep-tileserv"
  loadbalancer_id = azurerm_lb.main.id
}

# ── Health Probes ─────────────────────────────────────────────────────────────

resource "azurerm_lb_probe" "backend" {
  name            = "${var.app_name}-probe-backend"
  loadbalancer_id = azurerm_lb.main.id
  port            = var.backend_port
  protocol        = "Tcp"
  interval_in_seconds = 15
  number_of_probes    = 2
}

resource "azurerm_lb_probe" "backend_https" {
  name            = "${var.app_name}-probe-backend-https"
  loadbalancer_id = azurerm_lb.main.id
  port            = 443
  protocol        = "Tcp"
  interval_in_seconds = 15
  number_of_probes    = 2
}

resource "azurerm_lb_probe" "admin" {
  name            = "${var.app_name}-probe-admin"
  loadbalancer_id = azurerm_lb.main.id
  port            = var.backend_port
  protocol        = "Tcp"
  interval_in_seconds = 15
  number_of_probes    = 2
}

resource "azurerm_lb_probe" "tileserv" {
  name            = "${var.app_name}-probe-tileserv"
  loadbalancer_id = azurerm_lb.main.id
  port            = var.tileserv_nginx_port
  protocol        = "Tcp"
  interval_in_seconds = 30
  number_of_probes    = 2
}

# ── Load Balancing Rules ──────────────────────────────────────────────────────

# Port 80 → Backend (nginx handles HTTP→HTTPS redirect)
resource "azurerm_lb_rule" "backend" {
  name                    = "${var.app_name}-rule-backend"
  loadbalancer_id         = azurerm_lb.main.id
  protocol                = "Tcp"
  frontend_port           = 80
  backend_port            = 3000
  frontend_ip_configuration_name = "${var.app_name}-fe-lb"
  backend_address_pool_ids       = [azurerm_lb_backend_address_pool.backend.id]
  probe_id                       = azurerm_lb_probe.backend.id
}

# Port 443 → Backend (nginx terminates TLS)
resource "azurerm_lb_rule" "backend_https" {
  name                    = "${var.app_name}-rule-backend-https"
  loadbalancer_id         = azurerm_lb.main.id
  protocol                = "Tcp"
  frontend_port           = 443
  backend_port            = 443
  frontend_ip_configuration_name = "${var.app_name}-fe-lb"
  backend_address_pool_ids       = [azurerm_lb_backend_address_pool.backend.id]
  probe_id                       = azurerm_lb_probe.backend_https.id
}

# Port 3001 → Admin API
resource "azurerm_lb_rule" "admin" {
  name                    = "${var.app_name}-rule-admin"
  loadbalancer_id         = azurerm_lb.main.id
  protocol                = "Tcp"
  frontend_port           = 3001
  backend_port            = var.backend_port
  frontend_ip_configuration_name = "${var.app_name}-fe-lb"
  backend_address_pool_ids       = [azurerm_lb_backend_address_pool.admin.id]
  probe_id                       = azurerm_lb_probe.admin.id
}

# Port 8080 → pg_tileserv
resource "azurerm_lb_rule" "tileserv" {
  name                    = "${var.app_name}-rule-tileserv"
  loadbalancer_id         = azurerm_lb.main.id
  protocol                = "Tcp"
  frontend_port           = 8080
  backend_port            = var.tileserv_nginx_port
  frontend_ip_configuration_name = "${var.app_name}-fe-lb"
  backend_address_pool_ids       = [azurerm_lb_backend_address_pool.tileserv.id]
  probe_id                       = azurerm_lb_probe.tileserv.id
}
