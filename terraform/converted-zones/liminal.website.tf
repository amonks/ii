resource "aws_route53_zone" "liminal-website" {
  name = "liminal.website"
}

resource "aws_route53_record" "liminal-website-A" {
  zone_id = aws_route53_zone.liminal-website.zone_id
  name    = "liminal.website."
  type    = "A"
  ttl     = "300"
  records = ["192.30.252.153", "192.30.252.154"]
}

resource "aws_route53_record" "wildcard-liminal-website-A" {
  zone_id = aws_route53_zone.liminal-website.zone_id
  name    = "*.liminal.website."
  type    = "A"
  ttl     = "300"
  records = ["192.30.252.153", "192.30.252.154"]
}
