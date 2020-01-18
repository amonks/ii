resource "aws_route53_zone" "belgianman-com" {
  name = "belgianman.com"
}

resource "aws_route53_record" "www-belgianman-com-CNAME" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "www.belgianman.com."
  type    = "CNAME"
  ttl     = "300"
  records = ["wafelijzer.herokuapp.com."]
}

resource "aws_route53_record" "wafelijzer-belgianman-com-CNAME" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "wafelijzer.belgianman.com."
  type    = "CNAME"
  ttl     = "300"
  records = ["belgianman.github.io."]
}

resource "aws_route53_record" "status-belgianman-com-CNAME" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "status.belgianman.com."
  type    = "CNAME"
  ttl     = "300"
  records = ["stats.pingdom.com."]
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

resource "aws_route53_record" "belgianman-com-MX" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "belgianman.com."
  type    = "MX"
  ttl     = "28800"
  records = ["3 ALT1.ASPMX.L.GOOGLE.COM.", "3 ALT2.ASPMX.L.GOOGLE.COM.", "1 ASPMX.L.GOOGLE.COM.", "5 ASPMX2.GOOGLEMAIL.COM.", "5 ASPMX3.GOOGLEMAIL.COM."]
}

resource "aws_route53_record" "belgianman-com-A" {
  zone_id = aws_route53_zone.belgianman-com.zone_id
  name    = "belgianman.com."
  type    = "A"
  ttl     = "300"
  records = ["217.70.184.38"]
}
