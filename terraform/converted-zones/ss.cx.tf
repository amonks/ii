resource "aws_route53_zone" "ss-cx" {
  name = "ss.cx"
}

resource "aws_route53_record" "wifi-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "wifi.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["35.247.99.86"]
}

resource "aws_route53_record" "thor-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "thor.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.83.137.16"]
}

resource "aws_route53_record" "nencatacoa-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "nencatacoa.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.65.172.95"]
}

resource "aws_route53_record" "lugh-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "lugh.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.84.31.13"]
}

resource "aws_route53_record" "calder-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "calder.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.85.55.109"]
}

resource "aws_route53_record" "brigid-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "brigid.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.84.31.13"]
}

resource "aws_route53_record" "b-ss-cx-CNAME" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "b.ss.cx."
  type    = "CNAME"
  ttl     = "300"
  records = ["cross-origin-bouncer.herokuapp.com."]
}
