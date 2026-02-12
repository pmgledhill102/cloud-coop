# Terraform configuration for the cloudcoop integration test GCP project.
#
# This provisions the dedicated resources needed to run integration tests:
# - Required GCP APIs
# - Service account with minimal permissions
# - Budget alerts for cost control
#
# Usage:
#   cd integration/terraform
#   terraform init
#   terraform plan -var="project_id=cloudcoop-integration-test" -var="billing_account=XXXXXX-XXXXXX-XXXXXX"
#   terraform apply -var="project_id=cloudcoop-integration-test" -var="billing_account=XXXXXX-XXXXXX-XXXXXX"

terraform {
  required_version = ">= 1.5"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# Enable required APIs
resource "google_project_service" "compute" {
  service            = "compute.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "iam" {
  service            = "iam.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "logging" {
  service            = "logging.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "billing_budgets" {
  service            = "billingbudgets.googleapis.com"
  disable_on_destroy = false
}

# Service account for integration tests
resource "google_service_account" "integration_test" {
  account_id   = "cc-integration-test"
  display_name = "cloudcoop Integration Test"
  description  = "Service account for cloudcoop integration tests"
}

# Grant Compute Admin (manage VMs, firewall rules, disks)
resource "google_project_iam_member" "compute_admin" {
  project = var.project_id
  role    = "roles/compute.admin"
  member  = "serviceAccount:${google_service_account.integration_test.email}"
}

# Grant Service Account User (attach service accounts to VMs)
resource "google_project_iam_member" "sa_user" {
  project = var.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.integration_test.email}"
}

# Budget alert for cost control
resource "google_billing_budget" "integration_test" {
  count = var.billing_account != "" ? 1 : 0

  billing_account = var.billing_account
  display_name    = "cloudcoop Integration Test Budget"

  budget_filter {
    projects = ["projects/${var.project_id}"]
  }

  amount {
    specified_amount {
      currency_code = "USD"
      units         = var.monthly_budget_usd
    }
  }

  threshold_rules {
    threshold_percent = 0.5
  }
  threshold_rules {
    threshold_percent = 0.9
  }
  threshold_rules {
    threshold_percent = 1.0
  }
}
