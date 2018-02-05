resource "aws_route53_zone" "ss-cz" {
  name = "ss.cz"
}

resource "aws_route53_record" "v-ss-cz-CNAME" {
  zone_id = "${aws_route53_zone.ss-cz.zone_id}"
  name    = "v.ss.cz."
  type    = "CNAME"
  ttl     = "10800"
  records = ["amonks.github.io."]
}

resource "aws_route53_record" "ss-cz-A" {
  zone_id = "${aws_route53_zone.ss-cz.zone_id}"
  name    = "ss.cz."
  type    = "A"
  ttl     = "10800"
  records = ["168.235.71.126"]
}

resource "aws_route53_record" "s-ss-cz-CNAME" {
  zone_id = "${aws_route53_zone.ss-cz.zone_id}"
  name    = "s.ss.cz."
  type    = "CNAME"
  ttl     = "10800"
  records = ["bioart-saic-production.herokuapp.com."]
}

resource "aws_route53_record" "b-ss-cz-CNAME" {
  zone_id = "${aws_route53_zone.ss-cz.zone_id}"
  name    = "b.ss.cz."
  type    = "CNAME"
  ttl     = "10800"
  records = ["cross-origin-bouncer.herokuapp.com."]
}

resource "aws_route53_record" "0-ss-cz-CNAME" {
  zone_id = "${aws_route53_zone.ss-cz.zone_id}"
  name    = "0.ss.cz."
  type    = "CNAME"
  ttl     = "10800"
  records = ["0.ss.cx.s3.amazonaws.com."]
}
