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

resource "aws_route53_record" "qaynan-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "qaynan.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.87.233.36"]
}

resource "aws_route53_record" "oehlen-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "oehlen.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.113.84.10"]
}

resource "aws_route53_record" "lugh-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "lugh.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.84.171.132"]
}

resource "aws_route53_record" "go-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "go.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.84.224.155"]
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
  records = ["100.103.217.4"]
}

resource "aws_route53_record" "b-ss-cx-CNAME" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "b.ss.cx."
  type    = "CNAME"
  ttl     = "300"
  records = ["cross-origin-bouncer.herokuapp.com."]
}

resource "aws_route53_record" "wildcard-brigid-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "*.brigid.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.103.217.4"]
}
