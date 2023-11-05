resource "aws_route53_zone" "monks-co-public" {
  name = "monks.co"

  tags = {}
}

resource "aws_route53_record" "now-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "_now.monks.co"
  type    = "TXT"
  records = ["b3f39bbd3640c48b8335292201c1deff27525728caa8e8807b313040bbf78118"]
  ttl     = "300"
}

resource "aws_route53_record" "monks-co-A" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "monks.co"
  type    = "A"

  records = ["66.51.122.238"]
  ttl     = "300"
}

resource "aws_route53_record" "monks-co-AAAA" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "monks.co"
  type    = "AAAA"

  records = ["2a09:8280:1::f:478"]
  ttl     = "300"
}

resource "aws_route53_record" "monks-co-MX" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "monks.co"
  type    = "MX"
  records = ["10 in1-smtp.messagingengine.com.", "20 in2-smtp.messagingengine.com."]
  ttl     = "300"
}

resource "aws_route53_record" "monks-co-SPF" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "monks.co"
  type    = "SPF"
  records = ["v=spf1 include:spf.messagingengine.com include:spf.mandrillapp.com -all"]
  ttl     = "300"
}

resource "aws_route53_record" "monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "monks.co"
  type    = "TXT"
  records = ["v=spf1 include:spf.messagingengine.com include:spf.mandrillapp.com -all"]
  ttl     = "300"
}

resource "aws_route53_record" "_amazonses-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "_amazonses.monks.co"
  type    = "TXT"
  records = ["FhsJsbEIG/XDigqpB7eJufYKADoiedmLkiP1/kQT2k4="]
  ttl     = "1800"
}

resource "aws_route53_record" "a-6hnnvk6qxhx2zdw5diz3r6hkebqfnn4x-_domainkey-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "6hnnvk6qxhx2zdw5diz3r6hkebqfnn4x._domainkey.monks.co"
  type    = "CNAME"
  records = ["6hnnvk6qxhx2zdw5diz3r6hkebqfnn4x.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "fb3k3obomq6wzinzuf6bmslwjikic6vb-_domainkey-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "fb3k3obomq6wzinzuf6bmslwjikic6vb._domainkey.monks.co"
  type    = "CNAME"
  records = ["fb3k3obomq6wzinzuf6bmslwjikic6vb.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "kw44g4aoxriuk5f6dfgnscvdd627aopn-_domainkey-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "kw44g4aoxriuk5f6dfgnscvdd627aopn._domainkey.monks.co"
  type    = "CNAME"
  records = ["kw44g4aoxriuk5f6dfgnscvdd627aopn.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "mandrill-_domainkey-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "mandrill._domainkey.monks.co"
  type    = "TXT"
  records = ["v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCrLHiExVd55zd/IQ/J/mRwSRMAocV/hMB3jXwaHH36d9NaVynQFYV8NaWi69c1veUtRzGt7yAioXqLj7Z4TeEUoOLgrKsn8YnckGs9i3B3tVFB+Ch/4mPhXWiNfNdynHWBcPcbJ8kjEQ2U8y78dHZj1YeRXXVvWob2OaKynO8/lQIDAQAB;"]
  ttl     = "300"
}

resource "aws_route53_record" "fm1-_domainkey-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "fm1._domainkey.monks.co"
  type    = "CNAME"
  records = ["fm1.monks.co.dkim.fmhosted.com"]
  ttl     = "300"
}

resource "aws_route53_record" "fm2-_domainkey-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "fm2._domainkey.monks.co"
  type    = "CNAME"
  records = ["fm2.monks.co.dkim.fmhosted.com"]
  ttl     = "300"
}

resource "aws_route53_record" "fm3-_domainkey-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "fm3._domainkey.monks.co"
  type    = "CNAME"
  records = ["fm3.monks.co.dkim.fmhosted.com"]
  ttl     = "300"
}

resource "aws_route53_record" "_keybase-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "_keybase.monks.co"
  type    = "TXT"
  records = ["keybase-site-verification=JZj7vchXA6vfSV8oa5QQyGmnI8CKDRgQIHYIFPl5sF0"]
  ttl     = "300"
}

resource "aws_route53_record" "baton-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "baton.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "300"
}

resource "aws_route53_record" "f-monks-co-A" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "f.monks.co"
  type    = "A"

  alias {
    name                   = "d14nz3dle8w6lj.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "fftjs-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "fftjs.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "300"
}

resource "aws_route53_record" "graviton-monks-co-A" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "graviton.monks.co"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "installation-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "installation.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "300"
}

resource "aws_route53_record" "lyrics-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "lyrics.monks.co"
  type    = "CNAME"
  records = ["limitless-harbor-4493.herokuapp.com."]
  ttl     = "300"
}

resource "aws_route53_record" "monument-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "monument.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "300"
}

resource "aws_route53_record" "nabu-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "nabu.monks.co"
  type    = "CNAME"
  records = ["nabudata.herokuapp.com."]
  ttl     = "300"
}

resource "aws_route53_record" "numbers-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "numbers.monks.co"
  type    = "CNAME"
  records = ["tranquil-spire-3396.herokuapp.com."]
  ttl     = "300"
}

resource "aws_route53_record" "oblique-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "oblique.monks.co"
  type    = "CNAME"
  records = ["oblique-strategies-api.herokuapp.com."]
  ttl     = "300"
}

resource "aws_route53_record" "presence-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "presence.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "300"
}

resource "aws_route53_record" "processing-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co-public.zone_id
  name    = "processing.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "300"
}

