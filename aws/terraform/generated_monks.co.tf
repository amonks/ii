resource "aws_route53_zone" "monks-co" {
  name = "monks.co"
}

resource "aws_route53_record" "www-monks-co-AAAA" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "www.monks.co."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "www-monks-co-A" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "www.monks.co."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}

resource "aws_route53_record" "processing-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "processing.monks.co."
  type    = "CNAME"
  ttl     = "300"
  records = ["amonks.github.io."]
}

resource "aws_route53_record" "monks-co-SPF" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "monks.co."
  type    = "SPF"
  ttl     = "300"
  records = ["v=spf1 include:spf.messagingengine.com ?all"]
}

resource "aws_route53_record" "monks-co-MX" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "monks.co."
  type    = "MX"
  ttl     = "300"
  records = ["10 in1-smtp.messagingengine.com.", "20 in2-smtp.messagingengine.com."]
}

resource "aws_route53_record" "monks-co-AAAA" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "monks.co."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "monks-co-A" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "monks.co."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}

resource "aws_route53_record" "fm3-_domainkey-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "fm3._domainkey.monks.co."
  type    = "CNAME"
  ttl     = "300"
  records = ["fm3.monks.co.dkim.fmhosted.com."]
}

resource "aws_route53_record" "fm2-_domainkey-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "fm2._domainkey.monks.co."
  type    = "CNAME"
  ttl     = "300"
  records = ["fm2.monks.co.dkim.fmhosted.com."]
}

resource "aws_route53_record" "fm1-_domainkey-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "fm1._domainkey.monks.co."
  type    = "CNAME"
  ttl     = "300"
  records = ["fm1.monks.co.dkim.fmhosted.com."]
}

resource "aws_route53_record" "_keybase-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "_keybase.monks.co."
  type    = "TXT"
  ttl     = "300"
  records = ["keybase-site-verification=JZj7vchXA6vfSV8oa5QQyGmnI8CKDRgQIHYIFPl5sF0"]
}
