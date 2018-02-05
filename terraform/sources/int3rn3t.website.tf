resource "aws_route53_zone" "int3rn3t-website-public" {
  name    = "int3rn3t.website"

  tags {}
}

resource "aws_route53_record" "int3rn3t-website-A" {
  zone_id = "${aws_route53_zone.int3rn3t-website-public.zone_id}"
  name    = "int3rn3t.website"
  type    = "A"
  records = ["192.30.252.154", "192.30.252.153"]
  ttl     = "300"
}

resource "aws_route53_record" "int3rn3t-website-NS" {
  zone_id = "${aws_route53_zone.int3rn3t-website-public.zone_id}"
  name    = "int3rn3t.website"
  type    = "NS"
  records = ["ns-175.awsdns-21.com", "ns-1092.awsdns-08.org", "ns-1639.awsdns-12.co.uk", "ns-959.awsdns-55.net"]
  ttl     = "30"
}

resource "aws_route53_record" "int3rn3t-website-SOA" {
  zone_id = "${aws_route53_zone.int3rn3t-website-public.zone_id}"
  name    = "int3rn3t.website"
  type    = "SOA"
  records = ["ns-175.awsdns-21.com. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}

resource "aws_route53_record" "science-int3rn3t-website-A" {
  zone_id = "${aws_route53_zone.int3rn3t-website-public.zone_id}"
  name    = "science.int3rn3t.website"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "zingularity-int3rn3t-website-A" {
  zone_id = "${aws_route53_zone.int3rn3t-website-public.zone_id}"
  name    = "zingularity.int3rn3t.website"
  type    = "A"

  alias {
    name                   = "d1pqf9r4iv73tz.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}
