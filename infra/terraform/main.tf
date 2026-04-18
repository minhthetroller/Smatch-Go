terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# tflocal automatically overrides all provider endpoints to http://localhost:4566.
# No manual endpoint_url blocks needed.
provider "aws" {
  region  = var.aws_region
  profile = var.aws_profile # uses ~/.aws/credentials

  default_tags {
    tags = {
      Project = "smatch"
      Env     = var.environment
    }
  }
}
