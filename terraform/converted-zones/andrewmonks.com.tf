resource "aws_route53_zone" "andrewmonks-com" {
  name = "andrewmonks.com"
}

resource "aws_route53_record" "andrewmonks-com-A" {
  zone_id = aws_route53_zone.andrewmonks-com.zone_id
  name    = "andrewmonks.com."
  type    = "A"
  ttl     = "300"
  records = ["217.70.184.38"]
}
