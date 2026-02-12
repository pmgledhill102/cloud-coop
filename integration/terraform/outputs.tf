output "service_account_email" {
  description = "Email of the integration test service account"
  value       = google_service_account.integration_test.email
}

output "project_id" {
  description = "GCP project ID"
  value       = var.project_id
}

output "zone" {
  description = "GCP zone for tests"
  value       = var.zone
}
