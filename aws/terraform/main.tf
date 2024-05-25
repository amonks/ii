provider "aws" {
  region = "us-east-1"
}

terraform {
  backend "s3" {
    bucket = "monks-co-tfstate"
    key    = "terraform.tfstate"
    region = "us-east-1"

    dynamodb_table = "monks-co-tfstate-lock"
    encrypt        = true
  }
}

module "ss-cx-mailer" {
  source = "./mailer/"

  domain  = "ss.cx"
  zone_id = aws_route53_zone.ss-cx.id
}

terraform {
  required_providers {
    gandi = {
      version = ">= 2.1.0"
      source  = "go-gandi/gandi"
    }
  }
}

provider "gandi" {
  key = "FqMaynWbzxKKhj56kDaNcXX1"
}

output "monks-go_iam_user_access_key_id" {
  value = aws_iam_access_key.monks-go.id
}

output "monks-go_iam_user_secret_access_key" {
  value     = aws_iam_access_key.monks-go.secret
  sensitive = true
}

output "smtp_username" {
  value = module.ss-cx-mailer.smtp_username
}

output "smtp_password" {
  value     = module.ss-cx-mailer.smtp_password
  sensitive = true
}
