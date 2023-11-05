resource "aws_route53_zone" "needsyourhelp-org" {
  name = "needsyourhelp.org"
}

resource "aws_route53_record" "www-needsyourhelp-org-AAAA" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "www.needsyourhelp.org."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "www-needsyourhelp-org-A" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "www.needsyourhelp.org."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}

resource "aws_route53_record" "needsyourhelp-org-AAAA" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "needsyourhelp.org."
  type    = "AAAA"
  ttl     = "300"
  records = ["2a09:8280:1::f:478"]
}

resource "aws_route53_record" "needsyourhelp-org-A" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "needsyourhelp.org."
  type    = "A"
  ttl     = "300"
  records = ["66.51.122.238"]
}

resource "aws_route53_record" "doge-needsyourhelp-org-CNAME" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "doge.needsyourhelp.org."
  type    = "CNAME"
  ttl     = "300"
  records = ["amonks.github.io."]
}

resource "aws_route53_record" "divvy-needsyourhelp-org-CNAME" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "divvy.needsyourhelp.org."
  type    = "CNAME"
  ttl     = "300"
  records = ["divvy-json.herokuapp.com."]
}

resource "aws_route53_record" "brianeno-needsyourhelp-org-CNAME" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "brianeno.needsyourhelp.org."
  type    = "CNAME"
  ttl     = "300"
  records = ["oblique-strategies-api.herokuapp.com."]
}

resource "aws_route53_record" "bandcamp-needsyourhelp-org-CNAME" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "bandcamp.needsyourhelp.org."
  type    = "CNAME"
  ttl     = "300"
  records = ["bandcamp-pirate.herokuapp.com."]
}
