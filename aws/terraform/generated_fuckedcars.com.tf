resource "aws_route53_zone" "fuckedcars-com" {
  name = "fuckedcars.com"
}

resource "aws_route53_record" "www-fuckedcars-com-AAAA" {
  zone_id = aws_route53_zone.fuckedcars-com.zone_id
  name    = "www.fuckedcars.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "www-fuckedcars-com-A" {
  zone_id = aws_route53_zone.fuckedcars-com.zone_id
  name    = "www.fuckedcars.com."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}

resource "aws_route53_record" "fuckedcars-com-AAAA" {
  zone_id = aws_route53_zone.fuckedcars-com.zone_id
  name    = "fuckedcars.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "fuckedcars-com-A" {
  zone_id = aws_route53_zone.fuckedcars-com.zone_id
  name    = "fuckedcars.com."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}
