terraform {
  required_version = ">= 1.6"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.4"
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

provider "aws" {
  alias   = "us_east_1"
  region  = "us-east-1"
  profile = var.aws_profile

  default_tags {
    tags = {
      Project = "smatch"
      Env     = var.environment
    }
  }
}
