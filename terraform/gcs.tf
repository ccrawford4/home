resource "google_storage_bucket" "terraform_state_bucket" {
  name = "tf-state-home-prod"
  location = var.region

  lifecycle_rule {
    action {
      type = "Delete"
    }

    condition {
      age = 0
      days_since_custom_time = 0
      days_since_noncurrent_time = 0
      matches_prefix = []
      matches_storage_class = []
      matches_suffix = []
      no_age = false
      num_newer_versions = 2
      send_age_if_zero = true
      send_days_since_custom_time_if_zero = false
      send_days_since_noncurrent_time_if_zero = false
      send_num_newer_versions_if_zero = false
      with_state = "ARCHIVED"
    }
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }

    condition {
      age = 0
      days_since_custom_time = 0
      days_since_noncurrent_time = 7
      matches_prefix = []
      matches_storage_class = []
      matches_suffix = []
      no_age = false
      num_newer_versions = 0
      send_age_if_zero = true
      send_days_since_custom_time_if_zero = false
      send_days_since_noncurrent_time_if_zero = false
      send_num_newer_versions_if_zero = false
      with_state = "ANY"
    }
  }
}
