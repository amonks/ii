resource "aws_route53_zone" "blgn-mn" {
  name = "blgn.mn"
}

resource "aws_route53_record" "blgn-mn-AAAA" {
  zone_id = aws_route53_zone.blgn-mn.zone_id
  name    = "blgn.mn."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "blgn-mn-A" {
  zone_id = aws_route53_zone.blgn-mn.zone_id
  name    = "blgn.mn."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}
