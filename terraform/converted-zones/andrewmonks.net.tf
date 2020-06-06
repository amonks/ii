resource "aws_route53_zone" "andrewmonks-net" {
  name = "andrewmonks.net"
}

resource "aws_route53_record" "yungfuture-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "yungfuture.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["monks.co."]
}

resource "aws_route53_record" "www-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "www.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["monks.co."]
}

resource "aws_route53_record" "oblique-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "oblique.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["oblique-strategies-api.herokuapp.com."]
}

resource "aws_route53_record" "numbers-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "numbers.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["tranquil-spire-3396.herokuapp.com."]
}

resource "aws_route53_record" "lyrics-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "lyrics.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["limitless-harbor-4493.herokuapp.com."]
}

resource "aws_route53_record" "homer-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "homer.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["collectivememory.herokuapp.com."]
}

resource "aws_route53_record" "fm3-_domainkey-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "fm3._domainkey.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["fm3.andrewmonks.net.dkim.fmhosted.com."]
}

resource "aws_route53_record" "fm2-_domainkey-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "fm2._domainkey.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["fm2.andrewmonks.net.dkim.fmhosted.com."]
}

resource "aws_route53_record" "fm1-_domainkey-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "fm1._domainkey.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["fm1.andrewmonks.net.dkim.fmhosted.com."]
}

resource "aws_route53_record" "facekov-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "facekov.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["facekov.herokuapp.com."]
}

resource "aws_route53_record" "andrewmonks-net-TXT" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "andrewmonks.net."
  type    = "TXT"
  ttl     = "300"
  records = ["v=spf1 include:spf.messagingengine.com -all"]
}

resource "aws_route53_record" "andrewmonks-net-SPF" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "andrewmonks.net."
  type    = "SPF"
  ttl     = "300"
  records = ["v=spf1 include:spf.messagingengine.com -all"]
}

resource "aws_route53_record" "andrewmonks-net-MX" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "andrewmonks.net."
  type    = "MX"
  ttl     = "300"
  records = ["10 in1-smtp.messagingengine.com.", "20 in2-smtp.messagingengine.com."]
}

resource "aws_route53_record" "andrewmonks-net-A" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "andrewmonks.net."
  type    = "A"
  ttl     = "300"
  records = ["217.70.184.38"]
}

resource "aws_route53_record" "a-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "a.andrewmonks.net."
  type    = "CNAME"
  ttl     = "300"
  records = ["monks.co."]
}
