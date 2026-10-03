# Temporary: records where resources moved in the module refactor so Terraform
# re-addresses them instead of destroying and recreating them. Safe to delete
# once this has been applied.

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-db-username"].google_secret_manager_secret.secret
  to   = module.workload["search-app"].google_secret_manager_secret.this["search-app-db-username"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-db-username"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["search-app"].google_secret_manager_secret_iam_binding.accessor["search-app-db-username"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-db-password"].google_secret_manager_secret.secret
  to   = module.workload["search-app"].google_secret_manager_secret.this["search-app-db-password"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-db-password"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["search-app"].google_secret_manager_secret_iam_binding.accessor["search-app-db-password"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-db-root-password"].google_secret_manager_secret.secret
  to   = module.workload["search-app"].google_secret_manager_secret.this["search-app-db-root-password"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-db-root-password"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["search-app"].google_secret_manager_secret_iam_binding.accessor["search-app-db-root-password"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-redis-password"].google_secret_manager_secret.secret
  to   = module.workload["search-app"].google_secret_manager_secret.this["search-app-redis-password"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-redis-password"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["search-app"].google_secret_manager_secret_iam_binding.accessor["search-app-redis-password"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-nextauth-secret"].google_secret_manager_secret.secret
  to   = module.workload["search-app"].google_secret_manager_secret.this["search-app-nextauth-secret"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-nextauth-secret"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["search-app"].google_secret_manager_secret_iam_binding.accessor["search-app-nextauth-secret"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-github-id"].google_secret_manager_secret.secret
  to   = module.workload["search-app"].google_secret_manager_secret.this["search-app-github-id"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-github-id"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["search-app"].google_secret_manager_secret_iam_binding.accessor["search-app-github-id"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-github-secret"].google_secret_manager_secret.secret
  to   = module.workload["search-app"].google_secret_manager_secret.this["search-app-github-secret"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-github-secret"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["search-app"].google_secret_manager_secret_iam_binding.accessor["search-app-github-secret"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-google-id"].google_secret_manager_secret.secret
  to   = module.workload["search-app"].google_secret_manager_secret.this["search-app-google-id"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-google-id"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["search-app"].google_secret_manager_secret_iam_binding.accessor["search-app-google-id"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-google-secret"].google_secret_manager_secret.secret
  to   = module.workload["search-app"].google_secret_manager_secret.this["search-app-google-secret"]
}

moved {
  from = module.search-app-secrets.module.secrets_iam_binding["search-app-google-secret"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["search-app"].google_secret_manager_secret_iam_binding.accessor["search-app-google-secret"]
}

moved {
  from = module.ai-agent-api-secrets.module.secrets_iam_binding["ai-agent-api-chat-api-key"].google_secret_manager_secret.secret
  to   = module.workload["ai-agent-api"].google_secret_manager_secret.this["ai-agent-api-chat-api-key"]
}

moved {
  from = module.ai-agent-api-secrets.module.secrets_iam_binding["ai-agent-api-chat-api-key"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["ai-agent-api"].google_secret_manager_secret_iam_binding.accessor["ai-agent-api-chat-api-key"]
}

moved {
  from = module.ai-agent-api-secrets.module.secrets_iam_binding["ai-agent-api-kube-api-server"].google_secret_manager_secret.secret
  to   = module.workload["ai-agent-api"].google_secret_manager_secret.this["ai-agent-api-kube-api-server"]
}

moved {
  from = module.ai-agent-api-secrets.module.secrets_iam_binding["ai-agent-api-kube-api-server"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["ai-agent-api"].google_secret_manager_secret_iam_binding.accessor["ai-agent-api-kube-api-server"]
}

moved {
  from = module.ai-agent-api-secrets.module.secrets_iam_binding["ai-agent-api-openai-api-key"].google_secret_manager_secret.secret
  to   = module.workload["ai-agent-api"].google_secret_manager_secret.this["ai-agent-api-openai-api-key"]
}

moved {
  from = module.ai-agent-api-secrets.module.secrets_iam_binding["ai-agent-api-openai-api-key"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["ai-agent-api"].google_secret_manager_secret_iam_binding.accessor["ai-agent-api-openai-api-key"]
}

moved {
  from = module.ai-agent-api-secrets.module.secrets_iam_binding["ai-agent-api-redis-password"].google_secret_manager_secret.secret
  to   = module.workload["ai-agent-api"].google_secret_manager_secret.this["ai-agent-api-redis-password"]
}

moved {
  from = module.ai-agent-api-secrets.module.secrets_iam_binding["ai-agent-api-redis-password"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["ai-agent-api"].google_secret_manager_secret_iam_binding.accessor["ai-agent-api-redis-password"]
}

moved {
  from = module.portfolio-secrets.module.secrets_iam_binding["portfolio-chat-api-key"].google_secret_manager_secret.secret
  to   = module.workload["portfolio"].google_secret_manager_secret.this["portfolio-chat-api-key"]
}

moved {
  from = module.portfolio-secrets.module.secrets_iam_binding["portfolio-chat-api-key"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["portfolio"].google_secret_manager_secret_iam_binding.accessor["portfolio-chat-api-key"]
}

moved {
  from = module.openid-server-secrets.module.secrets_iam_binding["openid-server-kubernetes-api-url"].google_secret_manager_secret.secret
  to   = module.workload["openid-server"].google_secret_manager_secret.this["openid-server-kubernetes-api-url"]
}

moved {
  from = module.openid-server-secrets.module.secrets_iam_binding["openid-server-kubernetes-api-url"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["openid-server"].google_secret_manager_secret_iam_binding.accessor["openid-server-kubernetes-api-url"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-github-token"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-github-token"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-github-token"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-github-token"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-github-webhook-secret"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-github-webhook-secret"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-github-webhook-secret"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-github-webhook-secret"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-github-app-id"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-github-app-id"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-github-app-id"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-github-app-id"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-github-app-key"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-github-app-key"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-github-app-key"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-github-app-key"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-gcp-project-id"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-gcp-project-id"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-gcp-project-id"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-gcp-project-id"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-gcp-project-number"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-gcp-project-number"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-gcp-project-number"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-gcp-project-number"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-k8s-issuer-uri"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-k8s-issuer-uri"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-k8s-issuer-uri"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-k8s-issuer-uri"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-region"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-region"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-region"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-region"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-api-token"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-cloudflare-api-token"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-api-token"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-cloudflare-api-token"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-account-id"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-cloudflare-account-id"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-account-id"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-cloudflare-account-id"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-tunnel-secret"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-cloudflare-tunnel-secret"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-tunnel-secret"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-cloudflare-tunnel-secret"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-email"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-cloudflare-email"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-email"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-cloudflare-email"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-k8s-server-ip"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-k8s-server-ip"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-k8s-server-ip"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-k8s-server-ip"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-zone-id"].google_secret_manager_secret.secret
  to   = module.workload["atlantis"].google_secret_manager_secret.this["atlantis-cloudflare-zone-id"]
}

moved {
  from = module.atlantis-secrets.module.secrets_iam_binding["atlantis-cloudflare-zone-id"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["atlantis"].google_secret_manager_secret_iam_binding.accessor["atlantis-cloudflare-zone-id"]
}

moved {
  from = module.tekton-secrets.module.secrets_iam_binding["tekton-pipelines-github-token"].google_secret_manager_secret.secret
  to   = module.workload["tekton-pipelines"].google_secret_manager_secret.this["tekton-pipelines-github-token"]
}

moved {
  from = module.tekton-secrets.module.secrets_iam_binding["tekton-pipelines-github-token"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["tekton-pipelines"].google_secret_manager_secret_iam_binding.accessor["tekton-pipelines-github-token"]
}

moved {
  from = module.virgo-secrets.module.secrets_iam_binding["virgo-webui-admin-password"].google_secret_manager_secret.secret
  to   = module.workload["virgo"].google_secret_manager_secret.this["virgo-webui-admin-password"]
}

moved {
  from = module.virgo-secrets.module.secrets_iam_binding["virgo-webui-admin-password"].google_secret_manager_secret_iam_binding.k8s_sa_secret_binding
  to   = module.workload["virgo"].google_secret_manager_secret_iam_binding.accessor["virgo-webui-admin-password"]
}

moved {
  from = module.search-app-secrets.google_service_account.service_account[0]
  to   = google_service_account.secrets_manager
}

moved {
  from = google_service_account_iam_member.workload_identity_binding
  to   = google_service_account_iam_member.home_cluster_sa_workload_identity["portfolio/nginx-example"]
}

moved {
  from = google_service_account_iam_member.workload_identity_binding_virgo
  to   = google_service_account_iam_member.home_cluster_sa_workload_identity["virgo/virgo"]
}

moved {
  from = google_service_account_iam_member.workload_identity_binding_atlantis
  to   = google_service_account_iam_member.home_cluster_sa_workload_identity["atlantis/atlantis"]
}

moved {
  from = cloudflare_zero_trust_access_policy.home_master_k8s_api_admin
  to   = cloudflare_zero_trust_access_policy.this["admin"]
}

moved {
  from = cloudflare_zero_trust_access_policy.atlantis_admin
  to   = cloudflare_zero_trust_access_policy.this["atlantis_admin"]
}

moved {
  from = cloudflare_zero_trust_access_policy.atlantis_webhook_bypass
  to   = cloudflare_zero_trust_access_policy.this["atlantis_webhook_bypass"]
}

moved {
  from = cloudflare_dns_record.argocd
  to   = module.public_hostname["argocd"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_dns_record.search
  to   = module.public_hostname["search"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_dns_record.about
  to   = module.public_hostname["about"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_dns_record.openid
  to   = module.public_hostname["openid"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_dns_record.ollama
  to   = module.public_hostname["ollama"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_dns_record.atlantis
  to   = module.public_hostname["atlantis"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_dns_record.k8s
  to   = module.public_hostname["k8s"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_dns_record.ci
  to   = module.public_hostname["ci"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_dns_record.ai
  to   = module.public_hostname["ai"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_dns_record.chat
  to   = module.public_hostname["chat"].cloudflare_dns_record.this[0]
}

moved {
  from = cloudflare_zero_trust_access_application.home_master_k8s_api
  to   = module.public_hostname["k8s"].cloudflare_zero_trust_access_application.this["home-master-k8s-api"]
}

moved {
  from = cloudflare_zero_trust_access_application.home_master_argocd
  to   = module.public_hostname["argocd"].cloudflare_zero_trust_access_application.this["home-master-argocd"]
}

moved {
  from = cloudflare_zero_trust_access_application.atlantis_webhooks
  to   = module.public_hostname["atlantis"].cloudflare_zero_trust_access_application.this["atlantis-webhooks"]
}

moved {
  from = cloudflare_zero_trust_access_application.atlantis_ui
  to   = module.public_hostname["atlantis"].cloudflare_zero_trust_access_application.this["atlantis-ui"]
}

moved {
  from = google_service_account.home_cluster_sa
  to   = module.workload["atlantis"].google_service_account.this[0]
}

moved {
  from = google_service_account_iam_member.home_cluster_sa_workload_identity["atlantis/atlantis"]
  to   = module.workload["atlantis"].google_service_account_iam_member.workload_identity["atlantis"]
}

moved {
  from = google_storage_bucket_iam_member.atlantis_write_tf_state
  to   = module.workload["atlantis"].google_storage_bucket_iam_member.this["terraform-state"]
}
