resource "aws_route53_zone" "blgn-mn" {
  name = "blgn.mn"
}

resource "aws_route53_record" "www-blgn-mn-AAAA" {
  zone_id = aws_route53_zone.blgn-mn.zone_id
  name    = "www.blgn.mn."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "www-blgn-mn-A" {
  zone_id = aws_route53_zone.blgn-mn.zone_id
  name    = "www.blgn.mn."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "blgn-mn-AAAA" {
  zone_id = aws_route53_zone.blgn-mn.zone_id
  name    = "blgn.mn."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "blgn-mn-A" {
  zone_id = aws_route53_zone.blgn-mn.zone_id
  name    = "blgn.mn."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}
