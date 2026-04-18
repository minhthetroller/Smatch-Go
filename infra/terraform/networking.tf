# ── VPC ───────────────────────────────────────────────────────────────────────

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = { Name = "${var.app_name}-vpc" }
}

# ── Internet Gateway ──────────────────────────────────────────────────────────

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = { Name = "${var.app_name}-igw" }
}

# ── Public subnets (ALB) ─────────────────────────────────────────────────────

resource "aws_subnet" "public" {
  count             = length(var.public_subnet_cidrs)
  vpc_id            = aws_vpc.main.id
  cidr_block        = var.public_subnet_cidrs[count.index]
  availability_zone = var.availability_zones[count.index]

  map_public_ip_on_launch = true

  tags = { Name = "${var.app_name}-public-${count.index + 1}" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = { Name = "${var.app_name}-rt-public" }
}

resource "aws_route_table_association" "public" {
  count          = length(aws_subnet.public)
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# ── Private app subnets (ASG / EC2 instances) ─────────────────────────────────

resource "aws_subnet" "private_app" {
  count             = length(var.private_app_subnet_cidrs)
  vpc_id            = aws_vpc.main.id
  cidr_block        = var.private_app_subnet_cidrs[count.index]
  availability_zone = var.availability_zones[count.index]

  tags = { Name = "${var.app_name}-private-app-${count.index + 1}" }
}

resource "aws_route_table" "private_app" {
  vpc_id = aws_vpc.main.id

  tags = { Name = "${var.app_name}-rt-private-app" }
}

resource "aws_route" "private_app_nat" {
  count                  = var.aws_endpoint == "" ? 1 : 0
  route_table_id         = aws_route_table.private_app.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.main[0].id
}

resource "aws_route_table_association" "private_app" {
  count          = length(aws_subnet.private_app)
  subnet_id      = aws_subnet.private_app[count.index].id
  route_table_id = aws_route_table.private_app.id
}

# ── Private data subnets (RDS, ElastiCache) ───────────────────────────────────

resource "aws_subnet" "private" {
  count             = length(var.private_subnet_cidrs)
  vpc_id            = aws_vpc.main.id
  cidr_block        = var.private_subnet_cidrs[count.index]
  availability_zone = var.availability_zones[count.index]

  tags = { Name = "${var.app_name}-private-data-${count.index + 1}" }
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  tags = { Name = "${var.app_name}-rt-private-data" }
}

# NAT Gateway — allows private subnets outbound internet access
# Only created for real AWS deployments (not needed for LocalStack)
resource "aws_eip" "nat" {
  count  = var.aws_endpoint == "" ? 1 : 0
  domain = "vpc"
  tags   = { Name = "${var.app_name}-nat-eip" }
}

resource "aws_nat_gateway" "main" {
  count         = var.aws_endpoint == "" ? 1 : 0
  allocation_id = aws_eip.nat[0].id
  subnet_id     = aws_subnet.public[0].id
  tags          = { Name = "${var.app_name}-nat" }
  depends_on    = [aws_internet_gateway.main]
}

resource "aws_route" "private_nat" {
  count                  = var.aws_endpoint == "" ? 1 : 0
  route_table_id         = aws_route_table.private.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.main[0].id
}

resource "aws_route_table_association" "private" {
  count          = length(aws_subnet.private)
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

# ── Security Groups ───────────────────────────────────────────────────────────

# ALB — public HTTP/HTTPS access
resource "aws_security_group" "alb" {
  name        = "${var.app_name}-sg-alb"
  description = "Allow inbound HTTP to ALB"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.app_name}-sg-alb" }
}

# Backend — allow traffic from ALB only
resource "aws_security_group" "backend" {
  name        = "${var.app_name}-sg-backend"
  description = "Allow inbound from ALB to backend port"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Backend API"
    from_port       = var.backend_port
    to_port         = var.backend_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.app_name}-sg-backend" }
}

# RDS — allow only from backend
resource "aws_security_group" "rds" {
  name        = "${var.app_name}-sg-rds"
  description = "Allow Postgres from backend"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Postgres"
    from_port       = var.db_port
    to_port         = var.db_port
    protocol        = "tcp"
    security_groups = [aws_security_group.backend.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.app_name}-sg-rds" }
}

# Redis — allow only from backend
resource "aws_security_group" "redis" {
  name        = "${var.app_name}-sg-redis"
  description = "Allow Redis from backend"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Redis"
    from_port       = var.redis_port
    to_port         = var.redis_port
    protocol        = "tcp"
    security_groups = [aws_security_group.backend.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.app_name}-sg-redis" }
}
