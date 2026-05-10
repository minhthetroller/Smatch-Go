# ── Managed Identity for Backend VMSS ─────────────────────────────────────────

resource "azurerm_user_assigned_identity" "backend" {
  name                = "${var.app_name}-id-backend"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
}

# ── Backend VMSS ──────────────────────────────────────────────────────────────

resource "azurerm_linux_virtual_machine_scale_set" "backend" {
  name                 = "${var.app_name}-vmss-backend"
  resource_group_name  = azurerm_resource_group.main.name
  location             = azurerm_resource_group.main.location
  sku                  = var.vmss_instance_type
  instances            = var.backend_asg_desired_capacity
  admin_username       = var.vmss_admin_username
  upgrade_mode         = "Manual"
  health_probe_id      = azurerm_lb_probe.backend.id
  overprovision        = false

  admin_ssh_key {
    username   = var.vmss_admin_username
    public_key = var.vmss_ssh_public_key
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "ubuntu-24_04-lts"
    sku       = "server"
    version   = "latest"
  }

  os_disk {
    storage_account_type = "StandardSSD_LRS"
    caching              = "ReadWrite"
  }

  network_interface {
    name    = "${var.app_name}-nic-backend"
    primary = true

    ip_configuration {
      name      = "${var.app_name}-ipc-backend"
      primary   = true
      subnet_id = azurerm_subnet.private_app[0].id

      load_balancer_backend_address_pool_ids = [azurerm_lb_backend_address_pool.backend.id]
    }
  }

  identity {
    type         = "UserAssigned"
    identity_ids = [azurerm_user_assigned_identity.backend.id]
  }

  custom_data = base64encode(templatefile("${path.module}/cloud_init_backend.tpl", {
    backend_port                 = var.backend_port
    database_url                 = "postgresql://${var.db_username}:${var.db_password}@${azurerm_postgresql_flexible_server.main.fqdn}:${var.db_port}/${var.db_name}?sslmode=require"
    redis_host                   = azurerm_managed_redis.main.hostname
    redis_port                   = azurerm_managed_redis.main.default_database[0].port
    redis_password               = azurerm_managed_redis.main.default_database[0].primary_access_key
    storage_account_name         = azurerm_storage_account.main.name
    storage_account_key          = azurerm_storage_account.main.primary_access_key
    storage_container_profile    = var.storage_container_profile
    storage_container_matches    = var.storage_container_matches
    storage_container_business_docs = var.storage_container_business_docs
    acr_name                     = azurerm_container_registry.main.name
    acr_password                 = azurerm_container_registry.main.admin_password
    image_tag                    = "latest"
    cert_domain                  = var.domain_name != "" ? local.api_fqdn : "api-smatch.sbs"
    firebase_credentials_b64     = base64encode(file(var.firebase_credentials_file))
    zalopay_app_id               = var.zalopay_app_id
    zalopay_key1                 = var.zalopay_key1
    zalopay_key2                 = var.zalopay_key2
    zalopay_endpoint             = var.zalopay_endpoint
    zalopay_callback_url         = var.zalopay_callback_url
    tile_server_url              = var.tile_server_url
    admin_secret                 = var.admin_secret
    rate_limit_trusted_ips       = var.rate_limit_trusted_ips
    letsencrypt_email            = var.letsencrypt_email
  }))

  automatic_instance_repair {
    grace_period = "PT30M"
    enabled = true
  }

  tags = { Name = "${var.app_name}-vmss-backend" }
}

# ── Autoscale Settings ────────────────────────────────────────────────────────

resource "azurerm_monitor_autoscale_setting" "backend" {
  name                = "${var.app_name}-autoscale-backend"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  target_resource_id  = azurerm_linux_virtual_machine_scale_set.backend.id

  profile {
    name = "default"

    capacity {
      minimum = var.backend_asg_min_size
      maximum = var.backend_asg_max_size
      default = var.backend_asg_desired_capacity
    }

    rule {
      metric_trigger {
        metric_name        = "Percentage CPU"
        metric_resource_id = azurerm_linux_virtual_machine_scale_set.backend.id
        time_grain         = "PT1M"
        statistic          = "Average"
        time_window        = "PT5M"
        time_aggregation   = "Average"
        operator           = "GreaterThan"
        threshold          = 75
      }

      scale_action {
        direction = "Increase"
        type      = "ChangeCount"
        value     = "1"
        cooldown  = "PT1M"
      }
    }

    rule {
      metric_trigger {
        metric_name        = "Percentage CPU"
        metric_resource_id = azurerm_linux_virtual_machine_scale_set.backend.id
        time_grain         = "PT1M"
        statistic          = "Average"
        time_window        = "PT5M"
        time_aggregation   = "Average"
        operator           = "LessThan"
        threshold          = 25
      }

      scale_action {
        direction = "Decrease"
        type      = "ChangeCount"
        value     = "1"
        cooldown  = "PT1M"
      }
    }
  }
}
