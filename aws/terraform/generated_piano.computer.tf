resource "aws_route53_zone" "piano-computer" {
  name = "piano.computer"
}

resource "aws_route53_record" "www-piano-computer-AAAA" {
  zone_id = aws_route53_zone.piano-computer.zone_id
  name    = "www.piano.computer."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::2d:80c7:0"]
}

resource "aws_route53_record" "www-piano-computer-A" {
  zone_id = aws_route53_zone.piano-computer.zone_id
  name    = "www.piano.computer."
  type    = "A"
  ttl     = "300"
  records = ["137.66.9.17"]
}

resource "aws_route53_record" "piano-computer-AAAA" {
  zone_id = aws_route53_zone.piano-computer.zone_id
  name    = "piano.computer."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::2d:80c7:0"]
}

resource "aws_route53_record" "piano-computer-A" {
  zone_id = aws_route53_zone.piano-computer.zone_id
  name    = "piano.computer."
  type    = "A"
  ttl     = "300"
  records = ["137.66.9.17"]
}
