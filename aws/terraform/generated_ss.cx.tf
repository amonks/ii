resource "aws_route53_zone" "ss-cx" {
  name = "ss.cx"
}

resource "aws_route53_record" "www-ss-cx-AAAA" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "www.ss.cx."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "www-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "www.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}

resource "aws_route53_record" "wifi-ss-cx-CNAME" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "wifi.ss.cx."
  type    = "CNAME"
  ttl     = "300"
  records = ["p428.cloudunifi.com.ss.cx."]
}

resource "aws_route53_record" "w-ss-cx-CNAME" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "w.ss.cx."
  type    = "CNAME"
  ttl     = "300"
  records = ["lemon.whatbox.ca.ss.cx."]
}

resource "aws_route53_record" "thor-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "thor.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.83.137.16"]
}

resource "aws_route53_record" "ss-cx-AAAA" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "ss.cx."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}

resource "aws_route53_record" "qaynan-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "qaynan.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.82.39.94"]
}

resource "aws_route53_record" "lugh-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "lugh.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.85.208.103"]
}

resource "aws_route53_record" "go-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "go.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.84.224.155"]
}

resource "aws_route53_record" "fly-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "fly.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.84.224.155"]
}

resource "aws_route53_record" "calder-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "calder.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["100.126.181.120"]
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
