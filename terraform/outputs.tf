output "ecr_repository_url" {
  value = aws_ecr_repository.app.repository_url
}

output "github_actions_role_arn" {
  value = aws_iam_role.github_actions.arn
}

output "alb_dns_name" {
  value       = aws_lb.main.dns_name
  description = "Public DNS name of the Application Load Balancer"
}
