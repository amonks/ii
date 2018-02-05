domains
=======

## what

- terraform/sources contains my existing route53 zones and records, exported into .tf files using terraforming
- zones contains some zone files pasted in from gandi
- terraform/converted-zones contains the result of converting those zone files into .tf files

## how
	
- run `yarn convert` to convert zones to converted-zones
- run `yarn format` to format the terraform files
- run `yarn plan` to see what applying would do
- run `yarn apply` to deploy the config to aws
- run `yarn destroy` to delete all dns resources

