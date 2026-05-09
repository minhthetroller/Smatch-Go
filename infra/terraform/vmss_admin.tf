# ── Managed Identity for Admin VMSS ────────────────────────────────────────────

resource "azurerm_user_assigned_identity" "admin" {
  name                = "${var.app_name}-id-admin"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
}

# ── Admin VMSS ────────────────────────────────────────────────────────────────

resource "azurerm_linux_virtual_machine_scale_set" "admin" {
  name                 = "${var.app_name}-vmss-admin"
  resource_group_name  = azurerm_resource_group.main.name
  location             = azurerm_resource_group.main.location
  sku                  = var.vmss_instance_type
  instances            = var.admin_asg_desired_capacity
  admin_username       = var.vmss_admin_username
  upgrade_mode         = "Rolling"
  health_probe_id      = azurerm_lb_probe.admin.id
  overprovision        = false

  rolling_upgrade_policy {
    max_batch_instance_percent              = 50
    max_unhealthy_instance_percent          = 50
    max_unhealthy_upgraded_instance_percent = 50
    pause_time_between_batches              = "PT30S"
  }

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
    name    = "${var.app_name}-nic-admin"
    primary = true

    ip_configuration {
      name      = "${var.app_name}-ipc-admin"
      primary   = true
      subnet_id = azurerm_subnet.private_app[0].id

      load_balancer_backend_address_pool_ids = [azurerm_lb_backend_address_pool.admin.id]
    }
  }

  identity {
    type         = "UserAssigned"
    identity_ids = [azurerm_user_assigned_identity.admin.id]
  }

  custom_data = base64encode(templatefile("${path.module}/cloud_init_backend.tpl", {
    backend_port                 = var.backend_port
    database_url                 = "postgresql://${var.db_username}:${var.db_password}@${azurerm_postgresql_flexible_server.main.fqdn}:${var.db_port}/${var.db_name}?sslmode=require"
    redis_host                   = azurerm_redis_cache.main.hostname
    redis_port                   = azurerm_redis_cache.main.ssl_port
    redis_password               = azurerm_redis_cache.main.primary_access_key
    storage_account_name         = azurerm_storage_account.main.name
    storage_account_key          = azurerm_storage_account.main.primary_access_key
    storage_container_profile    = var.storage_container_profile
    storage_container_matches    = var.storage_container_matches
    storage_container_business_docs = var.storage_container_business_docs
    acr_name                     = azurerm_container_registry.main.name
    acr_password                 = azurerm_container_registry.main.admin_password
    image_tag                    = "admin"
    cert_domain                  = var.admin_domain_name != "" ? var.admin_domain_name : "api-smatch.sbs"
    firebase_credentials_b64     = base64encode(file(var.firebase_credentials_file))
    zalopay_app_id               = var.zalopay_app_id
    zalopay_key1                 = var.zalopay_key1
    zalopay_key2                 = var.zalopay_key2
    zalopay_endpoint             = var.zalopay_endpoint
    zalopay_callback_url         = var.zalopay_callback_url
    tile_server_url              = var.tile_server_url
    admin_secret                 = var.admin_secret
    rate_limit_trusted_ips       = var.rate_limit_trusted_ips
  }))

  automatic_instance_repair {
    grace_period = "PT30M"
    enabled = true
  }

  tags = { Name = "${var.app_name}-vmss-admin" }
}

resource "azurerm_monitor_autoscale_setting" "admin" {
  name                = "${var.app_name}-autoscale-admin"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  target_resource_id  = azurerm_linux_virtual_machine_scale_set.admin.id

  profile {
    name = "default"

    capacity {
      minimum = var.admin_asg_min_size
      maximum = var.admin_asg_max_size
      default = var.admin_asg_desired_capacity
    }

    rule {
      metric_trigger {
        metric_name        = "Percentage CPU"
        metric_resource_id = azurerm_linux_virtual_machine_scale_set.admin.id
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
        metric_resource_id = azurerm_linux_virtual_machine_scale_set.admin.id
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
