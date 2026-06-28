# ── RDS Read Replica ──────────────────────────────────────────────────────────

resource "aws_db_instance" "replica" {
  identifier          = "${var.app_name}-postgres-replica"
  replicate_source_db = aws_db_instance.main.identifier
  instance_class      = "db.t3.micro"
  publicly_accessible = false
  skip_final_snapshot = true

  vpc_security_group_ids = [aws_security_group.rds.id]

  tags = { Name = "${var.app_name}-rds-replica" }
}

# ── RDS Proxy Security Group ──────────────────────────────────────────────────
# Allows port 5432 only from the tileserv security group.

resource "aws_security_group" "rds_proxy" {
  name        = "${var.app_name}-sg-rds-proxy"
  description = "Allow Postgres from tileserv tasks to RDS Proxy"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Postgres from tileserv"
    from_port       = var.db_port
    to_port         = var.db_port
    protocol        = "tcp"
    security_groups = [aws_security_group.tileserv.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.app_name}-sg-rds-proxy" }
}

# ── IAM Role for RDS Proxy ────────────────────────────────────────────────────

resource "aws_iam_role" "rds_proxy" {
  name = "${var.app_name}-rds-proxy-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "rds.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = { Name = "${var.app_name}-rds-proxy-role" }
}

resource "aws_iam_role_policy" "rds_proxy_secrets" {
  name = "${var.app_name}-rds-proxy-secrets"
  role = aws_iam_role.rds_proxy.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue"]
      Resource = [aws_secretsmanager_secret.db_credentials.arn]
    }]
  })
}

# ── Secrets Manager: DB credentials (required by RDS Proxy) ──────────────────

resource "aws_secretsmanager_secret" "db_credentials" {
  name                    = "${var.app_name}/db/credentials"
  recovery_window_in_days = 0

  tags = { Name = "${var.app_name}-db-secret" }
}

resource "aws_secretsmanager_secret_version" "db_credentials" {
  secret_id = aws_secretsmanager_secret.db_credentials.id
  secret_string = jsonencode({
    username = var.db_username
    password = var.db_password
  })
}

# ── RDS Proxy ─────────────────────────────────────────────────────────────────

resource "aws_db_proxy" "tileserv" {
  name                   = "${var.app_name}-rds-proxy-tileserv"
  debug_logging          = false
  engine_family          = "POSTGRESQL"
  idle_client_timeout    = 300
  require_tls            = true
  role_arn               = aws_iam_role.rds_proxy.arn
  vpc_security_group_ids = [aws_security_group.rds_proxy.id]
  vpc_subnet_ids         = aws_subnet.private[*].id

  auth {
    auth_scheme = "SECRETS"
    iam_auth    = "DISABLED"
    secret_arn  = aws_secretsmanager_secret.db_credentials.arn
  }

  tags = { Name = "${var.app_name}-rds-proxy-tileserv" }
}

resource "aws_db_proxy_default_target_group" "tileserv" {
  db_proxy_name = aws_db_proxy.tileserv.name

  connection_pool_config {
    max_connections_percent      = 80
    max_idle_connections_percent = 30
    connection_borrow_timeout    = 120
  }
}

resource "aws_db_proxy_target" "tileserv" {
  db_proxy_name          = aws_db_proxy.tileserv.name
  target_group_name      = aws_db_proxy_default_target_group.tileserv.name
  db_instance_identifier = aws_db_instance.main.identifier
}
