resource "aws_route53_zone" "onyxrenewables-com-public" {
  name    = "onyxrenewables.com"
  comment = ""

  tags {}
}

resource "aws_route53_record" "onyxrenewables-com-NS" {
  zone_id = "${aws_route53_zone.onyxrenewables-com-public.zone_id}"
  name    = "onyxrenewables.com"
  type    = "NS"
  records = ["ns-1520.awsdns-62.org", "ns-627.awsdns-14.net", "ns-199.awsdns-24.com", "ns-1759.awsdns-27.co.uk"]
  ttl     = "172800"
}

resource "aws_route53_record" "onyxrenewables-com-SOA" {
  zone_id = "${aws_route53_zone.onyxrenewables-com-public.zone_id}"
  name    = "onyxrenewables.com"
  type    = "SOA"
  records = ["ns-1520.awsdns-62.org. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}

