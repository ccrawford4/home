terraform {
  backend "gcs" {
    bucket = "tf-state-home-prod" # Replace with your GCS bucket name
    prefix = "terraform/state"
  }

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }

  required_version = ">= 1.5.7"
}
