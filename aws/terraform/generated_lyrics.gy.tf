resource "aws_route53_zone" "lyrics-gy" {
  name = "lyrics.gy"
}

resource "aws_route53_record" "www-lyrics-gy-AAAA" {
  zone_id = aws_route53_zone.lyrics-gy.zone_id
  name    = "www.lyrics.gy."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "www-lyrics-gy-A" {
  zone_id = aws_route53_zone.lyrics-gy.zone_id
  name    = "www.lyrics.gy."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "lyrics-gy-AAAA" {
  zone_id = aws_route53_zone.lyrics-gy.zone_id
  name    = "lyrics.gy."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "lyrics-gy-A" {
  zone_id = aws_route53_zone.lyrics-gy.zone_id
  name    = "lyrics.gy."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}
