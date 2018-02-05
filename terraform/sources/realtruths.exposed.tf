resource "aws_route53_zone" "realtruths-exposed-public" {
  name    = "realtruths.exposed"
  comment = "HostedZone created by Route53 Registrar"

  tags {}
}

resource "aws_route53_record" "realtruths-exposed-NS" {
  zone_id = "${aws_route53_zone.realtruths-exposed-public.zone_id}"
  name    = "realtruths.exposed"
  type    = "NS"
  records = ["ns-1119.awsdns-11.org.", "ns-115.awsdns-14.com.", "ns-569.awsdns-07.net.", "ns-1724.awsdns-23.co.uk."]
  ttl     = "172800"
}

resource "aws_route53_record" "realtruths-exposed-SOA" {
  zone_id = "${aws_route53_zone.realtruths-exposed-public.zone_id}"
  name    = "realtruths.exposed"
  type    = "SOA"
  records = ["ns-1119.awsdns-11.org. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}
