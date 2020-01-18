resource "aws_route53_zone" "blgn-mn-public" {
  name = "blgn.mn"

  tags = {}
}

resource "aws_s3_bucket" "blgn-mn-bucket" {
  bucket = "blgn.mn"
  acl    = "public-read"

  website {
    redirect_all_requests_to = "http://belgianman.com"
  }
}

resource "aws_route53_record" "blgn-mn-A" {
  zone_id = aws_route53_zone.blgn-mn-public.zone_id
  name    = "blgn.mn"
  type    = "A"

  alias {
    name                   = aws_s3_bucket.blgn-mn-bucket.website_domain
    zone_id                = aws_s3_bucket.blgn-mn-bucket.hosted_zone_id
    evaluate_target_health = false
  }
}
