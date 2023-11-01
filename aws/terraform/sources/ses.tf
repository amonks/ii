resource "aws_ses_domain_identity" "ss-cx" {
  domain = "ss.cx"
}

resource "aws_route53_record" "sscx_amazonses_verification_record" {
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "_amazonses.ss.cx"
  type    = "TXT"
  ttl     = "600"
  records = [aws_ses_domain_identity.ss-cx.verification_token]
}

resource "aws_ses_domain_dkim" "ss-cx" {
  domain = aws_ses_domain_identity.ss-cx.domain
}

resource "aws_route53_record" "ss-cx_amazonses_dkim_record" {
  count   = 3
  zone_id = aws_route53_zone.ss-cx.zone_id
  name    = "${aws_ses_domain_dkim.ss-cx.dkim_tokens[count.index]}._domainkey"
  type    = "CNAME"
  ttl     = "600"
  records = ["${aws_ses_domain_dkim.ss-cx.dkim_tokens[count.index]}.dkim.amazonses.com"]
}

resource "aws_ses_domain_mail_from" "ss-cx" {
  domain           = aws_ses_domain_identity.ss-cx.domain
  mail_from_domain = "bounce.${aws_ses_domain_identity.ss-cx.domain}"
}

resource "aws_route53_record" "ss-cx_ses_domain_mail_from_mx" {
  zone_id = aws_route53_zone.ss-cx.id
  name    = aws_ses_domain_mail_from.ss-cx.mail_from_domain
  type    = "MX"
  ttl     = "600"
  records = ["10 feedback-smtp.us-east-1.amazonses.com"]
}

resource "aws_route53_record" "ss-cx_ses_domain_mail_from_txt" {
  zone_id = aws_route53_zone.ss-cx.id
  name    = aws_ses_domain_mail_from.ss-cx.mail_from_domain
  type    = "TXT"
  ttl     = "600"
  records = ["v=spf1 include:amazonses.com -all"]
}
