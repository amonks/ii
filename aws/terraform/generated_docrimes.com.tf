resource "aws_route53_zone" "docrimes-com" {
  name = "docrimes.com"
}

resource "aws_route53_record" "www-docrimes-com-AAAA" {
  zone_id = aws_route53_zone.docrimes-com.zone_id
  name    = "www.docrimes.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "www-docrimes-com-A" {
  zone_id = aws_route53_zone.docrimes-com.zone_id
  name    = "www.docrimes.com."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "docrimes-com-AAAA" {
  zone_id = aws_route53_zone.docrimes-com.zone_id
  name    = "docrimes.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "docrimes-com-A" {
  zone_id = aws_route53_zone.docrimes-com.zone_id
  name    = "docrimes.com."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}
