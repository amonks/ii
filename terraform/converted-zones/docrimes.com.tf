resource "aws_route53_zone" "docrimes-com" {
  name = "docrimes.com"
}

resource "aws_route53_record" "docrimes-com-A" {
  zone_id = aws_route53_zone.docrimes-com.zone_id
  name    = "docrimes.com."
  type    = "A"
  ttl     = "10800"
  records = ["192.30.252.153", "192.30.252.154"]
}

resource "aws_route53_record" "wildcard-docrimes-com-A" {
  zone_id = aws_route53_zone.docrimes-com.zone_id
  name    = "*.docrimes.com."
  type    = "A"
  ttl     = "10800"
  records = ["192.30.252.153", "192.30.252.154"]
}
