# cloudcoop Integration Test Project
#
# Copy this file to: gcp-org-management/layers/3-projects/staging/cloud-coop-inttest.tf
#
# Also update layers/3-projects/staging/outputs.tf to add:
#   cloud_coop_inttest = module.cloud_coop_inttest.project_id
#
# Staging folder must allow external IPs (integration tests SSH via external IP).
# Check layers/2-environments/staging/ folder policies.
#
# After applying, authenticate locally with:
#   gcloud auth application-default login \
#     --impersonate-service-account=cc-integration-test@cloud-coop-inttest.iam.gserviceaccount.com
#
# For CI, add the cloud-coop repo to the WIF pool in layer 0-bootstrap.

# --- Project ---

module "cloud_coop_inttest" {
  source    = "../../../modules/project-with-budget"
  providers = { google.quota = google.quota }

  project_id         = "cloud-coop-inttest"
  project_name       = "cloud-coop-inttest"
  folder_id          = local.folder_id
  billing_account_id = local.billing_account_id
  budget_amount      = 25
  alert_thresholds   = [0.5, 0.8, 1.0]

  kill_switch_topic     = local.kill_switch_topic
  notification_channels = local.notification_channels

  apis = [
    "compute.googleapis.com",
    "iam.googleapis.com",
    "logging.googleapis.com",
  ]

  labels = {
    environment = "staging"
    purpose     = "integration-testing"
  }
}

# --- Service Account ---

module "inttest_sa" {
  source = "../../../modules/service-account"

  account_id   = "cc-integration-test"
  display_name = "cloudcoop Integration Test Runner"
  description  = "Runs cloudcoop integration tests (VM lifecycle, SSH, provisioning)"
  project_id   = module.cloud_coop_inttest.project_id

  project_roles = {
    (module.cloud_coop_inttest.project_id) = [
      "roles/compute.admin",
      "roles/iam.serviceAccountUser",
    ]
  }

  impersonators = [
    "user:paul@pmgledhill.com",
  ]
}

# --- VPC Network ---
# Org policy prevents default network creation, so we create a custom VPC.

resource "google_compute_network" "inttest" {
  project                 = module.cloud_coop_inttest.project_id
  name                    = "inttest"
  auto_create_subnetworks = false

  depends_on = [module.cloud_coop_inttest]
}

resource "google_compute_subnetwork" "inttest" {
  project       = module.cloud_coop_inttest.project_id
  name          = "inttest-europe-north2"
  ip_cidr_range = "10.0.0.0/24"
  region        = "europe-north2"
  network       = google_compute_network.inttest.id
}

# Allow SSH from anywhere (integration tests connect via external IP)
resource "google_compute_firewall" "inttest_allow_ssh" {
  project = module.cloud_coop_inttest.project_id
  name    = "inttest-allow-ssh"
  network = google_compute_network.inttest.name

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  source_ranges = ["0.0.0.0/0"]

  depends_on = [module.cloud_coop_inttest]
}

# Allow all internal traffic within the subnet
resource "google_compute_firewall" "inttest_allow_internal" {
  project = module.cloud_coop_inttest.project_id
  name    = "inttest-allow-internal"
  network = google_compute_network.inttest.name

  allow {
    protocol = "tcp"
  }
  allow {
    protocol = "udp"
  }
  allow {
    protocol = "icmp"
  }

  source_ranges = ["10.0.0.0/24"]

  depends_on = [module.cloud_coop_inttest]
}
