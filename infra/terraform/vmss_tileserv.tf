# ── pg_tileserv VMSS ──────────────────────────────────────────────────────────

resource "azurerm_linux_virtual_machine_scale_set" "tileserv" {
  name                 = "${var.app_name}-vmss-tileserv"
  resource_group_name  = azurerm_resource_group.main.name
  location             = azurerm_resource_group.main.location
  sku                  = var.vmss_instance_type
  instances            = var.tileserv_asg_desired_capacity
  admin_username       = var.vmss_admin_username
  upgrade_mode         = "Rolling"
  health_probe_id      = azurerm_lb_probe.tileserv.id
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
    name    = "${var.app_name}-nic-tileserv"
    primary = true

    ip_configuration {
      name      = "${var.app_name}-ipc-tileserv"
      primary   = true
      subnet_id = azurerm_subnet.private_app[0].id

      load_balancer_backend_address_pool_ids = [azurerm_lb_backend_address_pool.tileserv.id]
    }
  }

  identity {
    type = "SystemAssigned"
  }

  custom_data = base64encode(templatefile("${path.module}/cloud_init_tileserv.tpl", {
    tileserv_port        = var.tileserv_port
    tileserv_nginx_port  = var.tileserv_nginx_port
    pg_tileserv_version  = var.pg_tileserv_version
    database_url         = "postgresql://${var.db_username}:${var.db_password}@${azurerm_postgresql_flexible_server.main.fqdn}:${var.db_port}/${var.db_name}?sslmode=require"
  }))

  automatic_instance_repair {
    grace_period = "PT30M"
    enabled = true
  }

  tags = { Name = "${var.app_name}-vmss-tileserv" }
}

resource "azurerm_monitor_autoscale_setting" "tileserv" {
  name                = "${var.app_name}-autoscale-tileserv"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  target_resource_id  = azurerm_linux_virtual_machine_scale_set.tileserv.id

  profile {
    name = "default"

    capacity {
      minimum = var.tileserv_asg_min_size
      maximum = var.tileserv_asg_max_size
      default = var.tileserv_asg_desired_capacity
    }

    rule {
      metric_trigger {
        metric_name        = "Percentage CPU"
        metric_resource_id = azurerm_linux_virtual_machine_scale_set.tileserv.id
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
        metric_resource_id = azurerm_linux_virtual_machine_scale_set.tileserv.id
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
