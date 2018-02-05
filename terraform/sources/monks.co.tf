resource "aws_route53_zone" "monks-co-public" {
  name    = "monks.co"
  comment = ""

  tags {}
}

resource "aws_route53_record" "monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "monks.co"
  type    = "A"

  alias {
    name                   = "dkpobroa8zd0h.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "monks-co-MX" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "monks.co"
  type    = "MX"
  records = ["10 in1-smtp.messagingengine.com.", "20 in2-smtp.messagingengine.com."]
  ttl     = "10800"
}

resource "aws_route53_record" "monks-co-NS" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "monks.co"
  type    = "NS"
  records = ["ns-1637.awsdns-12.co.uk.", "ns-168.awsdns-21.com.", "ns-800.awsdns-36.net.", "ns-1256.awsdns-29.org."]
  ttl     = "172800"
}

resource "aws_route53_record" "monks-co-SOA" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "monks.co"
  type    = "SOA"
  records = ["ns-1637.awsdns-12.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}

resource "aws_route53_record" "monks-co-SPF" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "monks.co"
  type    = "SPF"
  records = ["v=spf1 include:spf.messagingengine.com include:spf.mandrillapp.com -all"]
  ttl     = "10800"
}

resource "aws_route53_record" "monks-co-TXT" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "monks.co"
  type    = "TXT"
  records = ["v=spf1 include:spf.messagingengine.com include:spf.mandrillapp.com -all"]
  ttl     = "10800"
}

resource "aws_route53_record" "_amazonses-monks-co-TXT" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "_amazonses.monks.co"
  type    = "TXT"
  records = ["FhsJsbEIG/XDigqpB7eJufYKADoiedmLkiP1/kQT2k4="]
  ttl     = "1800"
}

resource "aws_route53_record" "6hnnvk6qxhx2zdw5diz3r6hkebqfnn4x-_domainkey-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "6hnnvk6qxhx2zdw5diz3r6hkebqfnn4x._domainkey.monks.co"
  type    = "CNAME"
  records = ["6hnnvk6qxhx2zdw5diz3r6hkebqfnn4x.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "fb3k3obomq6wzinzuf6bmslwjikic6vb-_domainkey-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "fb3k3obomq6wzinzuf6bmslwjikic6vb._domainkey.monks.co"
  type    = "CNAME"
  records = ["fb3k3obomq6wzinzuf6bmslwjikic6vb.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "kw44g4aoxriuk5f6dfgnscvdd627aopn-_domainkey-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "kw44g4aoxriuk5f6dfgnscvdd627aopn._domainkey.monks.co"
  type    = "CNAME"
  records = ["kw44g4aoxriuk5f6dfgnscvdd627aopn.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "mandrill-_domainkey-monks-co-TXT" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "mandrill._domainkey.monks.co"
  type    = "TXT"
  records = ["v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCrLHiExVd55zd/IQ/J/mRwSRMAocV/hMB3jXwaHH36d9NaVynQFYV8NaWi69c1veUtRzGt7yAioXqLj7Z4TeEUoOLgrKsn8YnckGs9i3B3tVFB+Ch/4mPhXWiNfNdynHWBcPcbJ8kjEQ2U8y78dHZj1YeRXXVvWob2OaKynO8/lQIDAQAB;"]
  ttl     = "10800"
}

resource "aws_route53_record" "mesmtp-_domainkey-monks-co-TXT" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "mesmtp._domainkey.monks.co"
  type    = "TXT"
  records = ["v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCsdLt3xomT52Iewm5v1RSpRqpXA2vIghgAHNck63znFYCEOFgVyKMbznwdqvO83Dv0MSzzHpwoC2lIj7oHZaIGHQDdISJpmOsaQrhri+3VES7lhE0z+OrfUv6kQKAYpKxgzDXSAC+n0fcIilvpzVRyKwX6yIA2rhrUM7mb21hQ6wIDAQAB"]
  ttl     = "10800"
}

resource "aws_route53_record" "_keybase-monks-co-TXT" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "_keybase.monks.co"
  type    = "TXT"
  records = ["keybase-site-verification=JZj7vchXA6vfSV8oa5QQyGmnI8CKDRgQIHYIFPl5sF0"]
  ttl     = "10800"
}

resource "aws_route53_record" "a-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "a.monks.co"
  type    = "CNAME"
  records = ["monks.co."]
  ttl     = "10800"
}

resource "aws_route53_record" "baton-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "baton.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "10800"
}

resource "aws_route53_record" "code-monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "code.monks.co"
  type    = "A"
  records = ["159.203.152.27"]
  ttl     = "300"
}

resource "aws_route53_record" "f-monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "f.monks.co"
  type    = "A"

  alias {
    name                   = "d14nz3dle8w6lj.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "facekov-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "facekov.monks.co"
  type    = "CNAME"
  records = ["facekov.herokuapp.com."]
  ttl     = "10800"
}

resource "aws_route53_record" "fftjs-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "fftjs.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "10800"
}

resource "aws_route53_record" "gimme-monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "gimme.monks.co"
  type    = "A"

  alias {
    name                   = "d3kicode3ffc45.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "graviton-monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "graviton.monks.co"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "homer-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "homer.monks.co"
  type    = "CNAME"
  records = ["collectivememory.herokuapp.com."]
  ttl     = "10800"
}

resource "aws_route53_record" "installation-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "installation.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "10800"
}

resource "aws_route53_record" "lyrics-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "lyrics.monks.co"
  type    = "CNAME"
  records = ["limitless-harbor-4493.herokuapp.com."]
  ttl     = "10800"
}

resource "aws_route53_record" "monument-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "monument.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "10800"
}

resource "aws_route53_record" "nabu-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "nabu.monks.co"
  type    = "CNAME"
  records = ["nabudata.herokuapp.com."]
  ttl     = "10800"
}

resource "aws_route53_record" "numbers-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "numbers.monks.co"
  type    = "CNAME"
  records = ["tranquil-spire-3396.herokuapp.com."]
  ttl     = "10800"
}

resource "aws_route53_record" "oblique-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "oblique.monks.co"
  type    = "CNAME"
  records = ["oblique-strategies-api.herokuapp.com."]
  ttl     = "10800"
}

resource "aws_route53_record" "presence-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "presence.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "300"
}

resource "aws_route53_record" "processing-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "processing.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "10800"
}

resource "aws_route53_record" "real-monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "real.monks.co"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "realgifs-monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "realgifs.monks.co"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "realproxy-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "realproxy.monks.co"
  type    = "CNAME"
  records = ["this-time-its-real.herokuapp.com."]
  ttl     = "10800"
}

resource "aws_route53_record" "shout-monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "shout.monks.co"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "surveil-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "surveil.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "10800"
}

resource "aws_route53_record" "text-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "text.monks.co"
  type    = "CNAME"
  records = ["personaltextgen.herokuapp.com."]
  ttl     = "10800"
}

resource "aws_route53_record" "u-monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "u.monks.co"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "vj-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "vj.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "10800"
}

resource "aws_route53_record" "vjjs-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "vjjs.monks.co"
  type    = "CNAME"
  records = ["amonks.github.io."]
  ttl     = "10800"
}

resource "aws_route53_record" "www-monks-co-A" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "www.monks.co"
  type    = "A"

  alias {
    name                   = "dkpobroa8zd0h.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "yungfuture-monks-co-CNAME" {
  zone_id = "${aws_route53_zone.monks-co-public.zone_id}"
  name    = "yungfuture.monks.co"
  type    = "CNAME"
  records = ["monks.co."]
  ttl     = "10800"
}
