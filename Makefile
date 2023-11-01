zone_files = $(wildcard zones/*)
terraform_source_files = $(wildcard terraform/sources/*)


apply : terraform/.terraform terraform/converted-zones $(terraform_source_files)
	@echo "-- Applying terraform"
	cd terraform && terraform apply

format : terraform/converted-zones $(terraform_source_files)
	@echo "-- Formatting terraform"
	cd terraform && terraform fmt

plan : terraform/.terraform terraform/converted-zones $(terraform_source_files)
	@echo "-- Running terraform plan"
	cd terraform && terraform plan

destroy : terraform/.terraform
	@echo "-- Destroying all terraformed DNS"
	cd terraform && terraform destroy


terraform/converted-zones : $(zone_files)
	@echo "-- Converting zones to terraform"
	fish convert-zones.fish

terraform/.terraform :
	@echo "-- Initializing terraform"
	cd terraform && terraform init

