resource "aws_route53_zone" "reptiles-exposed-public" {
  name    = "reptiles.exposed"
  comment = "HostedZone created by Route53 Registrar"

  tags {}
}

resource "aws_route53_record" "reptiles-exposed-NS" {
  zone_id = "${aws_route53_zone.reptiles-exposed-public.zone_id}"
  name    = "reptiles.exposed"
  type    = "NS"
  records = ["ns-1355.awsdns-41.org.", "ns-1577.awsdns-05.co.uk.", "ns-843.awsdns-41.net.", "ns-474.awsdns-59.com."]
  ttl     = "172800"
}

resource "aws_route53_record" "reptiles-exposed-SOA" {
  zone_id = "${aws_route53_zone.reptiles-exposed-public.zone_id}"
  name    = "reptiles.exposed"
  type    = "SOA"
  records = ["ns-1355.awsdns-41.org. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}
