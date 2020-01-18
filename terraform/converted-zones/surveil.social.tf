resource "aws_route53_zone" "surveil-social" {
  name = "surveil.social"
}

resource "aws_route53_record" "www-surveil-social-CNAME" {
  zone_id = aws_route53_zone.surveil-social.zone_id
  name    = "www.surveil.social."
  type    = "CNAME"
  ttl     = "300"
  records = ["amonks.github.io."]
}

resource "aws_route53_record" "surveil-social-A" {
  zone_id = aws_route53_zone.surveil-social.zone_id
  name    = "surveil.social."
  type    = "A"
  ttl     = "300"
  records = ["192.30.252.154"]
}
