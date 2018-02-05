resource "aws_route53_zone" "theweb-exposed-public" {
  name    = "theweb.exposed"
  comment = "HostedZone created by Route53 Registrar"

  tags {}
}

resource "aws_route53_record" "theweb-exposed-NS" {
  zone_id = "${aws_route53_zone.theweb-exposed-public.zone_id}"
  name    = "theweb.exposed"
  type    = "NS"
  records = ["ns-349.awsdns-43.com.", "ns-1627.awsdns-11.co.uk.", "ns-982.awsdns-58.net.", "ns-1391.awsdns-45.org."]
  ttl     = "172800"
}

resource "aws_route53_record" "theweb-exposed-SOA" {
  zone_id = "${aws_route53_zone.theweb-exposed-public.zone_id}"
  name    = "theweb.exposed"
  type    = "SOA"
  records = ["ns-349.awsdns-43.com. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}
