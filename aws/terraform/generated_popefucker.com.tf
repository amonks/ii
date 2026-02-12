resource "aws_route53_zone" "popefucker-com" {
  name = "popefucker.com"
}

resource "aws_route53_record" "www-popefucker-com-AAAA" {
  zone_id = aws_route53_zone.popefucker-com.zone_id
  name    = "www.popefucker.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "www-popefucker-com-A" {
  zone_id = aws_route53_zone.popefucker-com.zone_id
  name    = "www.popefucker.com."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "popefucker-com-AAAA" {
  zone_id = aws_route53_zone.popefucker-com.zone_id
  name    = "popefucker.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "popefucker-com-A" {
  zone_id = aws_route53_zone.popefucker-com.zone_id
  name    = "popefucker.com."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}
