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

resource "aws_route53_record" "updates-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "updates.monks.co."
  type    = "CNAME"
  ttl     = "300"
  records = ["murmuring-bedbug-xc9fvm5sklejkr3tlrhfat80.herokudns.com."]
}

resource "aws_route53_record" "processing-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "processing.monks.co."
  type    = "CNAME"
  ttl     = "300"
  records = ["amonks.github.io."]
}

resource "aws_route53_record" "pm-bounces-monks-co-CNAME" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "pm-bounces.monks.co."
  type    = "CNAME"
  ttl     = "300"
  records = ["pm.mtasv.net."]
}

resource "aws_route53_record" "monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "monks.co."
  type    = "TXT"
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

resource "aws_route53_record" "_dmarc-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "_dmarc.monks.co."
  type    = "TXT"
  ttl     = "300"
  records = ["v=DMARC1; p=quarantine; rua=mailto:940f0c149d7a65a4c6b0f32e5246f6fa@inbound.postmarkapp.com; aspf=r; pct=100"]
}

resource "aws_route53_record" "_atproto-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "_atproto.monks.co."
  type    = "TXT"
  ttl     = "300"
  records = ["did=did:plc:yfekcz5g5oabfem5icfcjj3d"]
}

resource "aws_route53_record" "_20240417205709pm-_domainkey-monks-co-TXT" {
  zone_id = aws_route53_zone.monks-co.zone_id
  name    = "20240417205709pm._domainkey.monks.co."
  type    = "TXT"
  ttl     = "300"
  records = ["k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCs6NQjBs0P6Nzq6uJxle+3nf7doYeNr7MsxNYF/YfZsJ24X3RlEjsg79uZOLnrQMJ/bhxZSyxyP/52VC+NZawQwoemILsyDAC4nILz6SGt5YrEGEW/SdsSqjqkFUTu2fHBbJFLFYHb83RMIvk1sy5KDs+B8WNewI4p3pT+rfaKgwIDAQAB"]
}
