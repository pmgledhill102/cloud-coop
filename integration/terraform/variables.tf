variable "project_id" {
  description = "GCP project ID for integration tests"
  type        = string
}

variable "region" {
  description = "GCP region for resources"
  type        = string
  default     = "us-central1"
}

variable "zone" {
  description = "GCP zone for compute resources"
  type        = string
  default     = "us-central1-a"
}

variable "billing_account" {
  description = "Billing account ID for budget alerts (optional, format: XXXXXX-XXXXXX-XXXXXX)"
  type        = string
  default     = ""
}

variable "monthly_budget_usd" {
  description = "Monthly budget in USD"
  type        = number
  default     = 50
}
