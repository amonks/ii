resource "aws_route53_zone" "xn--wxa-art-public" {
  name = "xn--wxa.art"

  tags = {}
}

resource "aws_route53_record" "xn--wxa-art-A" {
  zone_id = aws_route53_zone.xn--wxa-art-public.zone_id
  name    = "xn--wxa.art"
  type    = "A"

	alias {
    name = aws_s3_bucket.www-xn--wxa-art.website_domain
    zone_id = aws_s3_bucket.www-xn--wxa-art.hosted_zone_id
		evaluate_target_health = false
	}
}

resource "aws_route53_record" "www-xn--wxa-art-CNAME" {
	zone_id = aws_route53_zone.xn--wxa-art-public.zone_id
  name    = "www.xn--wxa.art"
  type    = "CNAME"
  records = ["xn--wxa.art"]
  ttl     = "300"
}

resource "aws_s3_bucket" "www-xn--wxa-art" {
	bucket = "xn--wxa.art"
	acl = "public-read"
	website {
		redirect_all_requests_to = "https://monks.co"
	}
}
