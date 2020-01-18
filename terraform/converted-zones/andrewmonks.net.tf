resource "aws_route53_zone" "andrewmonks-net" {
  name = "andrewmonks.net"
}

resource "aws_route53_record" "yungfuture-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "yungfuture.andrewmonks.net."
  type    = "CNAME"
  ttl     = "10800"
  records = ["monks.co."]
}

resource "aws_route53_record" "www-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "www.andrewmonks.net."
  type    = "CNAME"
  ttl     = "10800"
  records = ["monks.co."]
}

resource "aws_route53_record" "oblique-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "oblique.andrewmonks.net."
  type    = "CNAME"
  ttl     = "10800"
  records = ["oblique-strategies-api.herokuapp.com."]
}

resource "aws_route53_record" "numbers-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "numbers.andrewmonks.net."
  type    = "CNAME"
  ttl     = "10800"
  records = ["tranquil-spire-3396.herokuapp.com."]
}

resource "aws_route53_record" "mesmtp-_domainkey-andrewmonks-net-TXT" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "mesmtp._domainkey.andrewmonks.net."
  type    = "TXT"
  ttl     = "10800"
  records = ["v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCpei/JKjIBw0Cdq+2AoqOYF2/chTf2J94qbJ/UTotlofS4xdLNkDlLEG7iXC4J/gJhlYxWWl2zImu7GqgqIzvCxu30nqFS35wILvcNGfFV/V5MFXCua2dTwJERcY3DP41UuiNAdbR4GapEjRDYMcDDpnWnVAyb0+qxVv+lkCMWBwIDAQAB"]
}

resource "aws_route53_record" "lyrics-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "lyrics.andrewmonks.net."
  type    = "CNAME"
  ttl     = "10800"
  records = ["limitless-harbor-4493.herokuapp.com."]
}

resource "aws_route53_record" "homer-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "homer.andrewmonks.net."
  type    = "CNAME"
  ttl     = "10800"
  records = ["collectivememory.herokuapp.com."]
}

resource "aws_route53_record" "facekov-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "facekov.andrewmonks.net."
  type    = "CNAME"
  ttl     = "10800"
  records = ["facekov.herokuapp.com."]
}

resource "aws_route53_record" "andrewmonks-net-TXT" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "andrewmonks.net."
  type    = "TXT"
  ttl     = "10800"
  records = ["v=spf1 include:spf.messagingengine.com -all"]
}

resource "aws_route53_record" "andrewmonks-net-SPF" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "andrewmonks.net."
  type    = "SPF"
  ttl     = "10800"
  records = ["v=spf1 include:spf.messagingengine.com -all"]
}

resource "aws_route53_record" "andrewmonks-net-MX" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "andrewmonks.net."
  type    = "MX"
  ttl     = "10800"
  records = ["10 in1-smtp.messagingengine.com.", "20 in2-smtp.messagingengine.com."]
}

resource "aws_route53_record" "andrewmonks-net-A" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "andrewmonks.net."
  type    = "A"
  ttl     = "10800"
  records = ["217.70.184.38"]
}

resource "aws_route53_record" "a-andrewmonks-net-CNAME" {
  zone_id = aws_route53_zone.andrewmonks-net.zone_id
  name    = "a.andrewmonks.net."
  type    = "CNAME"
  ttl     = "10800"
  records = ["monks.co."]
}
