resource "aws_route53_zone" "andrewmonks-org" {
  name = "andrewmonks.org"
}

resource "aws_route53_record" "andrewmonks-org-A" {
  zone_id = aws_route53_zone.andrewmonks-org.zone_id
  name    = "andrewmonks.org."
  type    = "A"
  ttl     = "300"
  records = ["217.70.184.38"]
}
