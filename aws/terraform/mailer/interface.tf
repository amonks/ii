variable "domain" {
  type = string
}

variable "zone_id" {
  type = string
}

output "smtp_username" {
  value = aws_iam_access_key.access_key.id
}

output "smtp_password" {
  value     = aws_iam_access_key.access_key.ses_smtp_password_v4
  sensitive = true
}


