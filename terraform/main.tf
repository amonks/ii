provider "aws" {
  region = "us-east-1"
}

module converted_zones {
  source = "./converted-zones/"
}

module sources {
  source = "./sources/"
}

output "monks-go_iam_user_access_key_id" {
  value = module.sources.monks-go_iam_user_access_key_id
}

output "monks-go_iam_user_secret_access_key" {
  value = module.sources.monks-go_iam_user_secret_access_key
  sensitive = true
}
