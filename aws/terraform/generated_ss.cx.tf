resource "aws_route53_zone" "ss-cx" {
  name = "ss.cx"
}

resource "aws_route53_record" "www-ss-cx-AAAA" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "www.ss.cx."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "www-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "www.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "wifi-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "wifi.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["104.168.77.185"]
}

resource "aws_route53_record" "w-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "w.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["72.21.17.85"]
}

resource "aws_route53_record" "thr-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "thr.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.93.23.97"]
}

resource "aws_route53_record" "thor-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "thor.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.93.23.97"]
}

resource "aws_route53_record" "ss-cx-AAAA" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "ss.cx."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "lugh-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "lugh.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.75.168.50"]
}

resource "aws_route53_record" "local-brigid-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "local-brigid.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["192.168.1.40"]
}

resource "aws_route53_record" "fly-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "fly.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.124.71.5"]
}

resource "aws_route53_record" "brigid-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "brigid.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.85.200.70"]
}

resource "aws_route53_record" "wildcard-brigid-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "*.brigid.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.85.200.70"]
}
