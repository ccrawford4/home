terraform {
  required_version = ">= 1.10"
  backend "gcs" {
    bucket = "tf-state-home-prod"
  }
}
