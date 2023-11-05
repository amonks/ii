resource "aws_route53_zone" "andrewmonks-org" {
  name = "andrewmonks.org"
}

resource "aws_route53_record" "www-andrewmonks-org-AAAA" {
  zone_id = aws_route53_zone.andrewmonks-org.zone_id
  name    = "www.andrewmonks.org."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "www-andrewmonks-org-A" {
  zone_id = aws_route53_zone.andrewmonks-org.zone_id
  name    = "www.andrewmonks.org."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}

resource "aws_route53_record" "andrewmonks-org-AAAA" {
  zone_id = aws_route53_zone.andrewmonks-org.zone_id
  name    = "andrewmonks.org."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "andrewmonks-org-A" {
  zone_id = aws_route53_zone.andrewmonks-org.zone_id
  name    = "andrewmonks.org."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}
