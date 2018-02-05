provider "aws" {
  region = "us-east-1"
}

module converted_zones {
  source = "./converted-zones/"
}

module sources {
  source = "./sources/"
}
