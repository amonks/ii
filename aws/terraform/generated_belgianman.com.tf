resource "aws_route53_zone" "belgianman-com" {
  name = "belgianman.com"
}

resource "aws_route53_record" "www-belgianman-com-AAAA" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "www.belgianman.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "www-belgianman-com-A" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "www.belgianman.com."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "old-belgianman-com-CNAME" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "old.belgianman.com."
  type    = "CNAME"
  ttl     = "300"
  records = ["belgianman.github.io."]
}

resource "aws_route53_record" "music-belgianman-com-CNAME" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "music.belgianman.com."
  type    = "CNAME"
  ttl     = "300"
  records = ["dom.bandcamp.com."]
}

resource "aws_route53_record" "belgianman-com-AAAA" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "belgianman.com."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "belgianman-com-A" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "belgianman.com."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}
