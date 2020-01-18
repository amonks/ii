resource "aws_route53_zone" "wunder-xyz" {
  name = "wunder.xyz"
}

resource "aws_route53_record" "wunder-xyz-A" {
  zone_id = aws_route53_zone.wunder-xyz.zone_id
  name    = "wunder.xyz."
  type    = "A"
  ttl     = "300"
  records = ["192.30.252.153", "192.30.252.154"]
}

resource "aws_route53_record" "wildcard-wunder-xyz-A" {
  zone_id = aws_route53_zone.wunder-xyz.zone_id
  name    = "*.wunder.xyz."
  type    = "A"
  ttl     = "300"
  records = ["192.30.252.153", "192.30.252.154"]
}
