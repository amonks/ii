resource "aws_route53_zone" "fmail-email" {
  name = "fmail.email"
}

resource "aws_route53_record" "www-fmail-email-AAAA" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "www.fmail.email."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "www-fmail-email-A" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "www.fmail.email."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "mesmtp-_domainkey-fmail-email-CNAME" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "mesmtp._domainkey.fmail.email."
  type    = "CNAME"
  ttl     = "300"
  records = ["mesmtp.fmail.email.dkim.fmhosted.com."]
}

resource "aws_route53_record" "mail-fmail-email-MX" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "mail.fmail.email."
  type    = "MX"
  ttl     = "300"
  records = ["10 in1-smtp.messagingengine.com.", "20 in2-smtp.messagingengine.com."]
}

resource "aws_route53_record" "fmail-email-TXT" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "fmail.email."
  type    = "TXT"
  ttl     = "300"
  records = ["v=spf1 include:spf.messagingengine.com -all"]
}

resource "aws_route53_record" "fmail-email-MX" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "fmail.email."
  type    = "MX"
  ttl     = "300"
  records = ["10 in1-smtp.messagingengine.com.", "20 in2-smtp.messagingengine.com."]
}

resource "aws_route53_record" "fmail-email-AAAA" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "fmail.email."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::d2:e9f9:0"]
}

resource "aws_route53_record" "fmail-email-A" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "fmail.email."
  type    = "A"
  ttl     = "300"
  records = ["37.16.29.186"]
}

resource "aws_route53_record" "fm3-_domainkey-fmail-email-CNAME" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "fm3._domainkey.fmail.email."
  type    = "CNAME"
  ttl     = "300"
  records = ["fm3.fmail.email.dkim.fmhosted.com."]
}

resource "aws_route53_record" "fm2-_domainkey-fmail-email-CNAME" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "fm2._domainkey.fmail.email."
  type    = "CNAME"
  ttl     = "300"
  records = ["fm2.fmail.email.dkim.fmhosted.com."]
}

resource "aws_route53_record" "fm1-_domainkey-fmail-email-CNAME" {
  zone_id = aws_route53_zone.fmail-email.zone_id
  name    = "fm1._domainkey.fmail.email."
  type    = "CNAME"
  ttl     = "300"
  records = ["fm1.fmail.email.dkim.fmhosted.com."]
}
