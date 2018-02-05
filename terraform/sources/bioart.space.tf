resource "aws_route53_zone" "bioart-space-public" {
  name    = "bioart.space"
  comment = "bioart"

  tags {}
}

resource "aws_route53_record" "bioart-space-A" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "bioart.space"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "bioart-space-NS" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "bioart.space"
  type    = "NS"
  records = ["ns-294.awsdns-36.com.", "ns-1053.awsdns-03.org.", "ns-1677.awsdns-17.co.uk.", "ns-912.awsdns-50.net."]
  ttl     = "172800"
}

resource "aws_route53_record" "bioart-space-SOA" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "bioart.space"
  type    = "SOA"
  records = ["ns-294.awsdns-36.com. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}

resource "aws_route53_record" "deployer-bioart-space-CNAME" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "deployer.bioart.space"
  type    = "CNAME"
  records = ["bioart-deploy.herokuapp.com"]
  ttl     = "300"
}

resource "aws_route53_record" "dewpoint-bioart-space-CNAME" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "dewpoint.bioart.space"
  type    = "CNAME"
  records = ["mauricehampton.github.io"]
  ttl     = "300"
}

resource "aws_route53_record" "www-bioart-space-A" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "www.bioart.space"
  type    = "A"

  alias {
    name                   = "bioart.space"
    zone_id                = "${aws_route53_zone.bioart-space-public.zone_id}"
    evaluate_target_health = false
  }
}
