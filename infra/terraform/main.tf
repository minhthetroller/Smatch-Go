terraform {
  required_version = ">= 1.6"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "azurerm" {
  features {}
}

resource "random_id" "suffix" {
  byte_length = 4
}

locals {
  api_fqdn = var.domain_name != "" ? (
    var.api_subdomain != "" ? "${var.api_subdomain}.${var.domain_name}" : var.domain_name
  ) : ""
}

resource "azurerm_resource_group" "main" {
  name     = "${var.app_name}-rg-${var.environment}"
  location = var.azure_region

  tags = {
    Project = "smatch"
    Env     = var.environment
  }
}
