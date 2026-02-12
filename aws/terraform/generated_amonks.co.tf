resource "aws_route53_zone" "amonks-co" {
  name = "amonks.co"
}

resource "aws_route53_record" "www-amonks-co-AAAA" {
  zone_id = aws_route53_zone.amonks-co.zone_id
  name    = "www.amonks.co."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "www-amonks-co-A" {
  zone_id = aws_route53_zone.amonks-co.zone_id
  name    = "www.amonks.co."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "amonks-co-AAAA" {
  zone_id = aws_route53_zone.amonks-co.zone_id
  name    = "amonks.co."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "amonks-co-A" {
  zone_id = aws_route53_zone.amonks-co.zone_id
  name    = "amonks.co."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}
