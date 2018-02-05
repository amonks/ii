resource "aws_route53_zone" "theweb-exposed-public" {
  name    = "theweb.exposed"
  comment = "HostedZone created by Route53 Registrar"

  tags {}
}

resource "aws_route53_record" "theweb-exposed-NS" {
  zone_id = "${aws_route53_zone.theweb-exposed-public.zone_id}"
  name    = "theweb.exposed"
  type    = "NS"
  records = ["ns-632.awsdns-15.net", "ns-1958.awsdns-52.co.uk", "ns-1204.awsdns-22.org", "ns-199.awsdns-24.com"]
  ttl     = "172800"
}

resource "aws_route53_record" "theweb-exposed-SOA" {
  zone_id = "${aws_route53_zone.theweb-exposed-public.zone_id}"
  name    = "theweb.exposed"
  type    = "SOA"
  records = ["ns-632.awsdns-15.net. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}
