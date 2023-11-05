resource "aws_route53_zone" "andrewmonks-com" {
  name = "andrewmonks.com"
}

resource "aws_route53_record" "www-andrewmonks-com-AAAA" {
  zone_id = aws_route53_zone.andrewmonks-com.zone_id
  name    = "www.andrewmonks.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "www-andrewmonks-com-A" {
  zone_id = aws_route53_zone.andrewmonks-com.zone_id
  name    = "www.andrewmonks.com."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}

resource "aws_route53_record" "andrewmonks-com-AAAA" {
  zone_id = aws_route53_zone.andrewmonks-com.zone_id
  name    = "andrewmonks.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "andrewmonks-com-A" {
  zone_id = aws_route53_zone.andrewmonks-com.zone_id
  name    = "andrewmonks.com."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}
