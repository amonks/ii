resource "aws_route53_zone" "needsyourhelp-org" {
  name = "needsyourhelp.org"
}

resource "aws_route53_record" "doge-needsyourhelp-org-CNAME" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "doge.needsyourhelp.org."
  type    = "CNAME"
  ttl     = "10800"
  records = ["amonks.github.io."]
}

resource "aws_route53_record" "divvy-needsyourhelp-org-CNAME" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "divvy.needsyourhelp.org."
  type    = "CNAME"
  ttl     = "10800"
  records = ["divvy-json.herokuapp.com."]
}

resource "aws_route53_record" "brianeno-needsyourhelp-org-CNAME" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "brianeno.needsyourhelp.org."
  type    = "CNAME"
  ttl     = "10800"
  records = ["oblique-strategies-api.herokuapp.com."]
}

resource "aws_route53_record" "bandcamp-needsyourhelp-org-CNAME" {
  zone_id = aws_route53_zone.needsyourhelp-org.zone_id
  name    = "bandcamp.needsyourhelp.org."
  type    = "CNAME"
  ttl     = "10800"
  records = ["bandcamp-pirate.herokuapp.com."]
}
