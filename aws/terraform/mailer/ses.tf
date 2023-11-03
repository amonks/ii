resource "aws_ses_domain_identity" "mailer" {
  domain = var.domain
}

resource "aws_ses_domain_mail_from" "mailer" {
  domain           = aws_ses_domain_identity.mailer.domain
  mail_from_domain = "mail.${aws_ses_domain_identity.mailer.domain}"
}

resource "aws_route53_record" "MX" {
  zone_id = var.zone_id
  name    = aws_ses_domain_mail_from.mailer.mail_from_domain
  type    = "MX"
  ttl     = "600"
  records = ["10 feedback-smtp.us-east-1.amazonses.com"]
}

resource "aws_route53_record" "SPF-TXT" {
  zone_id = var.zone_id
  name    = aws_ses_domain_mail_from.mailer.mail_from_domain
  type    = "TXT"
  ttl     = "300"
  records = ["v=spf1 include:amazonses.com ~all"]
}

resource "aws_ses_domain_dkim" "mailer" {
  domain = aws_ses_domain_identity.mailer.domain
}

resource "aws_route53_record" "DKIM" {
  count   = 3
  zone_id = var.zone_id
  name    = "${element(aws_ses_domain_dkim.mailer.dkim_tokens, count.index)}._domainkey.${aws_ses_domain_mail_from.mailer.domain}"
  type    = "CNAME"
  ttl     = "300"
  records = ["${element(aws_ses_domain_dkim.mailer.dkim_tokens, count.index)}.dkim.amazonses.com"]
}

resource "aws_route53_record" "TXT-verification" {
  zone_id = var.zone_id
  name    = "_amazonses.${aws_ses_domain_identity.mailer.domain}"
  type    = "TXT"
  ttl     = "300"
  records = [aws_ses_domain_identity.mailer.verification_token]
}

resource "aws_ses_domain_identity_verification" "mailer" {
  domain = aws_ses_domain_identity.mailer.id

  depends_on = [aws_route53_record.TXT-verification]
}
