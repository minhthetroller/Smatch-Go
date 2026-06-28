packer {
  required_version = ">= 1.10.0"

  required_plugins {
    amazon = {
      source  = "github.com/hashicorp/amazon"
      version = ">= 1.3.0"
    }
  }
}

variable "aws_region" {
  type    = string
  default = "ap-southeast-1"
}

variable "aws_profile" {
  type    = string
  default = "default"
}

variable "instance_type" {
  type    = string
  default = "t3.micro"
}

variable "ami_name_prefix" {
  type    = string
  default = "smatch-runtime-al2023"
}

variable "source_ami_architecture" {
  type    = string
  default = "x86_64"
}

source "amazon-ebs" "runtime" {
  region        = var.aws_region
  profile       = var.aws_profile
  instance_type = var.instance_type
  ssh_username  = "ec2-user"

  ami_name        = "${var.ami_name_prefix}-${formatdate("YYYYMMDDhhmmss", timestamp())}"
  ami_description = "Smatch AL2023 runtime AMI with Docker, CloudWatch Agent, SSM, and AWS CLI"

  source_ami_filter {
    filters = {
      architecture        = var.source_ami_architecture
      name                = "al2023-ami-2023*-kernel-6.1-${var.source_ami_architecture}"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    owners      = ["amazon"]
    most_recent = true
  }

  tags = {
    Name    = var.ami_name_prefix
    Project = "smatch"
    Role    = "runtime"
    BuiltBy = "packer"
  }
}

build {
  name    = "smatch-runtime"
  sources = ["source.amazon-ebs.runtime"]

  provisioner "shell" {
    inline = [
      "set -euo pipefail",
      "sudo dnf update -y",
      "sudo dnf install -y docker amazon-cloudwatch-agent",
      "if ! command -v aws >/dev/null 2>&1; then sudo dnf install -y awscli-2 || sudo dnf install -y awscli; fi",
      "sudo systemctl enable docker",
      "sudo systemctl enable amazon-ssm-agent || true",
      "sudo mkdir -p /opt/aws/amazon-cloudwatch-agent/etc",
      "sudo cloud-init clean --logs"
    ]
  }
}
