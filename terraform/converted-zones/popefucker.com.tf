resource "aws_route53_zone" "popefucker-com" {
  name = "popefucker.com"
}

resource "aws_route53_record" "popefucker-com-A" {
  zone_id = aws_route53_zone.popefucker-com.zone_id
  name    = "popefucker.com."
  type    = "A"
  ttl     = "300"
  records = ["159.203.95.137"]
}

resource "aws_route53_record" "wildcard-popefucker-com-A" {
  zone_id = aws_route53_zone.popefucker-com.zone_id
  name    = "*.popefucker.com."
  type    = "A"
  ttl     = "300"
  records = ["159.203.95.137"]
}
