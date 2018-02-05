resource "aws_route53_zone" "reptiles-exposed-public" {
  name    = "reptiles.exposed"

  tags {}
}

resource "aws_route53_record" "reptiles-exposed-NS" {
  zone_id = "${aws_route53_zone.reptiles-exposed-public.zone_id}"
  name    = "reptiles.exposed"
  type    = "NS"
  records = ["ns-144.awsdns-18.com", "ns-1810.awsdns-34.co.uk", "ns-1322.awsdns-37.org", "ns-929.awsdns-52.net"]
  ttl     = "172800"
}

resource "aws_route53_record" "reptiles-exposed-SOA" {
  zone_id = "${aws_route53_zone.reptiles-exposed-public.zone_id}"
  name    = "reptiles.exposed"
  type    = "SOA"
  records = ["ns-144.awsdns-18.com. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}
