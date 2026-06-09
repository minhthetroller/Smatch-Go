# ── CloudWatch Dashboard ─────────────────────────────────────────────────────

resource "aws_cloudwatch_dashboard" "infrastructure" {
  dashboard_name = "${var.app_name}-${var.environment}-infrastructure"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "text"
        x      = 0
        y      = 0
        width  = 24
        height = 2
        properties = {
          markdown = "# ${var.app_name} ${var.environment} infrastructure\nRDS, EC2 Auto Scaling groups, host-level CloudWatch Agent metrics, and ECS Fargate tileserv."
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 2
        width  = 12
        height = 6
        properties = {
          title   = "RDS CPU and Connections"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Average"
          metrics = [
            ["AWS/RDS", "CPUUtilization", "DBInstanceIdentifier", aws_db_instance.main.identifier, { label = "primary CPU %" }],
            [".", ".", ".", aws_db_instance.replica.identifier, { label = "replica CPU %" }],
            [".", "DatabaseConnections", ".", aws_db_instance.main.identifier, { label = "primary connections", yAxis = "right" }],
            [".", ".", ".", aws_db_instance.replica.identifier, { label = "replica connections", yAxis = "right" }]
          ]
          yAxis = {
            left  = { min = 0, max = 100, label = "CPU %" }
            right = { min = 0, label = "connections" }
          }
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 2
        width  = 12
        height = 6
        properties = {
          title   = "RDS Memory and Storage"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Average"
          metrics = [
            ["AWS/RDS", "FreeableMemory", "DBInstanceIdentifier", aws_db_instance.main.identifier, { label = "primary free memory", yAxis = "left" }],
            [".", ".", ".", aws_db_instance.replica.identifier, { label = "replica free memory", yAxis = "left" }],
            [".", "FreeStorageSpace", ".", aws_db_instance.main.identifier, { label = "primary free storage", yAxis = "right" }],
            [".", ".", ".", aws_db_instance.replica.identifier, { label = "replica free storage", yAxis = "right" }]
          ]
          yAxis = {
            left  = { label = "bytes" }
            right = { label = "bytes" }
          }
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 8
        width  = 12
        height = 6
        properties = {
          title   = "RDS IOPS"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Average"
          metrics = [
            ["AWS/RDS", "ReadIOPS", "DBInstanceIdentifier", aws_db_instance.main.identifier, { label = "primary read IOPS" }],
            [".", "WriteIOPS", ".", aws_db_instance.main.identifier, { label = "primary write IOPS" }],
            [".", "ReadIOPS", ".", aws_db_instance.replica.identifier, { label = "replica read IOPS" }],
            [".", "WriteIOPS", ".", aws_db_instance.replica.identifier, { label = "replica write IOPS" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 8
        width  = 12
        height = 6
        properties = {
          title   = "RDS Latency and Replica Lag"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Average"
          metrics = [
            ["AWS/RDS", "ReadLatency", "DBInstanceIdentifier", aws_db_instance.main.identifier, { label = "primary read latency" }],
            [".", "WriteLatency", ".", aws_db_instance.main.identifier, { label = "primary write latency" }],
            [".", "ReadLatency", ".", aws_db_instance.replica.identifier, { label = "replica read latency" }],
            [".", "WriteLatency", ".", aws_db_instance.replica.identifier, { label = "replica write latency" }],
            [".", "ReplicaLag", ".", aws_db_instance.replica.identifier, { label = "replica lag", yAxis = "right" }]
          ]
          yAxis = {
            left  = { label = "seconds" }
            right = { label = "seconds" }
          }
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 14
        width  = 12
        height = 6
        properties = {
          title   = "EC2 CPU by Auto Scaling Group"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Average"
          metrics = [
            ["AWS/EC2", "CPUUtilization", "AutoScalingGroupName", aws_autoscaling_group.backend.name, { label = "backend CPU %" }],
            [".", ".", ".", aws_autoscaling_group.admin.name, { label = "admin CPU %" }]
          ]
          yAxis = {
            left = { min = 0, max = 100 }
          }
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 14
        width  = 12
        height = 6
        properties = {
          title   = "EC2 Network by Auto Scaling Group"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Sum"
          metrics = [
            ["AWS/EC2", "NetworkIn", "AutoScalingGroupName", aws_autoscaling_group.backend.name, { label = "backend network in" }],
            [".", "NetworkOut", ".", aws_autoscaling_group.backend.name, { label = "backend network out" }],
            [".", "NetworkIn", ".", aws_autoscaling_group.admin.name, { label = "admin network in" }],
            [".", "NetworkOut", ".", aws_autoscaling_group.admin.name, { label = "admin network out" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 20
        width  = 12
        height = 6
        properties = {
          title   = "EC2 Memory by Auto Scaling Group"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Average"
          metrics = [
            ["CWAgent", "mem_used_percent", "AutoScalingGroupName", aws_autoscaling_group.backend.name, { label = "backend memory used %" }],
            [".", ".", ".", aws_autoscaling_group.admin.name, { label = "admin memory used %" }],
            [".", "mem_available_percent", ".", aws_autoscaling_group.backend.name, { label = "backend memory available %" }],
            [".", ".", ".", aws_autoscaling_group.admin.name, { label = "admin memory available %" }]
          ]
          yAxis = {
            left = { min = 0, max = 100 }
          }
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 20
        width  = 12
        height = 6
        properties = {
          title   = "EC2 Disk Usage by Auto Scaling Group"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Average"
          metrics = [
            ["CWAgent", "disk_used_percent", "AutoScalingGroupName", aws_autoscaling_group.backend.name, { label = "backend root disk used %" }],
            [".", ".", ".", aws_autoscaling_group.admin.name, { label = "admin root disk used %" }],
            [".", "disk_inodes_used", ".", aws_autoscaling_group.backend.name, { label = "backend inodes used", yAxis = "right" }],
            [".", ".", ".", aws_autoscaling_group.admin.name, { label = "admin inodes used", yAxis = "right" }]
          ]
          yAxis = {
            left  = { min = 0, max = 100 }
            right = { min = 0 }
          }
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 26
        width  = 12
        height = 6
        properties = {
          title   = "EC2 Disk I/O by Auto Scaling Group"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Sum"
          metrics = [
            ["AWS/EC2", "DiskReadBytes", "AutoScalingGroupName", aws_autoscaling_group.backend.name, { label = "backend EC2 read bytes" }],
            [".", "DiskWriteBytes", ".", aws_autoscaling_group.backend.name, { label = "backend EC2 write bytes" }],
            [".", "DiskReadBytes", ".", aws_autoscaling_group.admin.name, { label = "admin EC2 read bytes" }],
            [".", "DiskWriteBytes", ".", aws_autoscaling_group.admin.name, { label = "admin EC2 write bytes" }],
            ["CWAgent", "diskio_read_bytes", "AutoScalingGroupName", aws_autoscaling_group.backend.name, { label = "backend agent read bytes" }],
            [".", "diskio_write_bytes", ".", aws_autoscaling_group.backend.name, { label = "backend agent write bytes" }],
            [".", "diskio_read_bytes", ".", aws_autoscaling_group.admin.name, { label = "admin agent read bytes" }],
            [".", "diskio_write_bytes", ".", aws_autoscaling_group.admin.name, { label = "admin agent write bytes" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 26
        width  = 12
        height = 6
        properties = {
          title   = "ECS Fargate Service Utilization"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Average"
          metrics = [
            ["AWS/ECS", "CPUUtilization", "ClusterName", aws_ecs_cluster.tileserv.name, "ServiceName", aws_ecs_service.tileserv.name, { label = "tileserv CPU %" }],
            [".", "MemoryUtilization", ".", ".", ".", ".", { label = "tileserv memory %" }]
          ]
          yAxis = {
            left = { min = 0, max = 100 }
          }
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 32
        width  = 12
        height = 6
        properties = {
          title   = "ECS Fargate Container Insights"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = 60
          stat    = "Average"
          metrics = [
            ["ECS/ContainerInsights", "CpuUtilized", "ClusterName", aws_ecs_cluster.tileserv.name, "ServiceName", aws_ecs_service.tileserv.name, { label = "CPU utilized" }],
            [".", "MemoryUtilized", ".", ".", ".", ".", { label = "memory utilized" }],
            [".", "NetworkRxBytes", ".", ".", ".", ".", { label = "network rx bytes", stat = "Sum", yAxis = "right" }],
            [".", "NetworkTxBytes", ".", ".", ".", ".", { label = "network tx bytes", stat = "Sum", yAxis = "right" }]
          ]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 32
        width  = 12
        height = 6
        properties = {
          title   = "Backend Host CPU and Memory (10s CWAgent)"
          region  = var.aws_region
          view    = "timeSeries"
          stacked = false
          period  = var.backend_cloudwatch_metrics_collection_interval_seconds
          stat    = "Average"
          metrics = [
            ["CWAgent", "cpu_usage_idle", "AutoScalingGroupName", aws_autoscaling_group.backend.name, { id = "cpu_idle", visible = false }],
            [{ expression = "100 - cpu_idle", label = "backend host CPU used %", id = "cpu_used" }],
            ["CWAgent", "mem_used_percent", "AutoScalingGroupName", aws_autoscaling_group.backend.name, { label = "backend memory used %" }]
          ]
          yAxis = {
            left = { min = 0, max = 100 }
          }
        }
      }
    ]
  })
}
