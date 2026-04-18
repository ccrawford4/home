# Worker

1. Receives the Payload from a GitHub Webhook
2. Parse Relevant Information
   PR -> PR URL
   Calls Terraform Plan -> Success? Report Results with a Comment on the PR -> Failure = Comment on the PR with the Error Message
