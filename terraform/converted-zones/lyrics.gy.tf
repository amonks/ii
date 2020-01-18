resource "aws_route53_zone" "lyrics-gy" {
  name = "lyrics.gy"
}

resource "aws_route53_record" "www-lyrics-gy-CNAME" {
  zone_id = aws_route53_zone.lyrics-gy.zone_id
  name    = "www.lyrics.gy."
  type    = "CNAME"
  ttl     = "10800"
  records = ["lyricsgen.herokuapp.com."]
}

resource "aws_route53_record" "lyrics-gy-A" {
  zone_id = aws_route53_zone.lyrics-gy.zone_id
  name    = "lyrics.gy."
  type    = "A"
  ttl     = "10800"
  records = ["217.70.184.38"]
}
