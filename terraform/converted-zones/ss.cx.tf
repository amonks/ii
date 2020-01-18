resource "aws_route53_zone" "ss-cx" {
  name = "ss.cx"
}

resource "aws_route53_record" "x-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "x.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["34.83.45.46"]
}

resource "aws_route53_record" "wifi-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "wifi.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["35.247.99.86"]
}

resource "aws_route53_record" "vpn-ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "vpn.ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["35.230.108.149"]
}

resource "aws_route53_record" "v-ss-cx-CNAME" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "v.ss.cx."
  type    = "CNAME"
  ttl     = "300"
  records = ["amonks.github.io."]
}

resource "aws_route53_record" "ss-cx-A" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "ss.cx."
  type    = "A"
  ttl     = "300"
  records = ["168.235.71.126"]
}

resource "aws_route53_record" "s-ss-cx-CNAME" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "s.ss.cx."
  type    = "CNAME"
  ttl     = "300"
  records = ["bioart-saic-production.herokuapp.com."]
}

resource "aws_route53_record" "b-ss-cx-CNAME" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "b.ss.cx."
  type    = "CNAME"
  ttl     = "300"
  records = ["cross-origin-bouncer.herokuapp.com."]
}
