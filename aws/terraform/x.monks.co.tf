module "x-monks-co" {
  source  = "./public_bucket"
  name    = "x.monks.co"
  zone_id = aws_route53_zone.monks-co.zone_id
}

