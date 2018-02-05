resource "aws_route53_zone" "realtruths-exposed-public" {
  name    = "realtruths.exposed"

  tags {}
}

resource "aws_route53_record" "realtruths-exposed-NS" {
  zone_id = "${aws_route53_zone.realtruths-exposed-public.zone_id}"
  name    = "realtruths.exposed"
  type    = "NS"
  records = ["ns-1479.awsdns-56.org", "ns-161.awsdns-20.com", "ns-1011.awsdns-62.net", "ns-1556.awsdns-02.co.uk"]
  ttl     = "172800"
}

resource "aws_route53_record" "realtruths-exposed-SOA" {
  zone_id = "${aws_route53_zone.realtruths-exposed-public.zone_id}"
  name    = "realtruths.exposed"
  type    = "SOA"
  records = ["ns-1479.awsdns-56.org. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}
