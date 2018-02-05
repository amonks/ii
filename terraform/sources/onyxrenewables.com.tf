resource "aws_route53_zone" "onyxrenewables-com-public" {
  name    = "onyxrenewables.com"
  comment = ""

  tags {}
}

resource "aws_route53_record" "onyxrenewables-com-NS" {
  zone_id = "${aws_route53_zone.onyxrenewables-com-public.zone_id}"
  name    = "onyxrenewables.com"
  type    = "NS"
  records = ["ns-1268.awsdns-30.org.", "ns-515.awsdns-00.net.", "ns-1728.awsdns-24.co.uk.", "ns-24.awsdns-03.com."]
  ttl     = "172800"
}

resource "aws_route53_record" "onyxrenewables-com-SOA" {
  zone_id = "${aws_route53_zone.onyxrenewables-com-public.zone_id}"
  name    = "onyxrenewables.com"
  type    = "SOA"
  records = ["ns-1268.awsdns-30.org. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}
