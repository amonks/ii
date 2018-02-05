resource "aws_route53_zone" "xn--onks-99b-com-public" {
  name = "xn--onks-99b.com"

  tags {}
}

resource "aws_route53_record" "xn--onks-99b-com-A" {
  zone_id = "${aws_route53_zone.xn--onks-99b-com-public.zone_id}"
  name    = "xn--onks-99b.com"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "_keybase-xn--onks-99b-com-TXT" {
  zone_id = "${aws_route53_zone.xn--onks-99b-com-public.zone_id}"
  name    = "_keybase.xn--onks-99b.com"
  type    = "TXT"
  records = ["keybase-site-verification=FW8oH3s4U7E5u4mO1QbTMFboNeFpgVui6SHDdKgAEU4"]
  ttl     = "300"
}

resource "aws_route53_record" "l1zard-xn--onks-99b-com-A" {
  zone_id = "${aws_route53_zone.xn--onks-99b-com-public.zone_id}"
  name    = "l1zard.xn--onks-99b.com"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "www-xn--onks-99b-com-CNAME" {
  zone_id = "${aws_route53_zone.xn--onks-99b-com-public.zone_id}"
  name    = "www.xn--onks-99b.com"
  type    = "CNAME"
  records = ["monks.co"]
  ttl     = "300"
}
