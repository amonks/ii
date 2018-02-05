resource "aws_route53_zone" "bioart-space-public" {
  name = "bioart.space"

  tags {}
}

resource "aws_route53_record" "bioart-space-A" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "bioart.space"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "deployer-bioart-space-CNAME" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "deployer.bioart.space"
  type    = "CNAME"
  records = ["bioart-deploy.herokuapp.com"]
  ttl     = "300"
}

resource "aws_route53_record" "dewpoint-bioart-space-CNAME" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "dewpoint.bioart.space"
  type    = "CNAME"
  records = ["mauricehampton.github.io"]
  ttl     = "300"
}

resource "aws_route53_record" "www-bioart-space-A" {
  zone_id = "${aws_route53_zone.bioart-space-public.zone_id}"
  name    = "www.bioart.space"
  type    = "A"

  alias {
    name                   = "bioart.space"
    zone_id                = "${aws_route53_zone.bioart-space-public.zone_id}"
    evaluate_target_health = false
  }
}
