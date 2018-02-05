resource "aws_route53_zone" "gifbooth-co-public" {
  name    = "gifbooth.co"
  comment = "gifbooth"

  tags {}
}

resource "aws_route53_record" "gifbooth-co-A" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "gifbooth.co"
  type    = "A"

  alias {
    name                   = "d3pr1dprea0g5n.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "gifbooth-co-MX" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "gifbooth.co"
  type    = "MX"
  records = ["10 inbound-smtp.us-east-1.amazonaws.com."]
  ttl     = "300"
}

resource "aws_route53_record" "gifbooth-co-NS" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "gifbooth.co"
  type    = "NS"
  records = ["ns-769.awsdns-32.net.", "ns-1595.awsdns-07.co.uk.", "ns-390.awsdns-48.com.", "ns-1215.awsdns-23.org."]
  ttl     = "172800"
}

resource "aws_route53_record" "gifbooth-co-SOA" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "gifbooth.co"
  type    = "SOA"
  records = ["ns-769.awsdns-32.net. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}

resource "aws_route53_record" "_amazonses-gifbooth-co-TXT" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "_amazonses.gifbooth.co"
  type    = "TXT"
  records = ["rgV0IaEBkhwjbFau3iHbqgY6MnttGaeBPNQD1OZ5V90="]
  ttl     = "1800"
}

resource "aws_route53_record" "lo4od7zfwvqb2jmkjuqbe5yaza4eul43-_domainkey-gifbooth-co-CNAME" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "lo4od7zfwvqb2jmkjuqbe5yaza4eul43._domainkey.gifbooth.co"
  type    = "CNAME"
  records = ["lo4od7zfwvqb2jmkjuqbe5yaza4eul43.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "vo3lqx4fdwkk5ipwghlrbifnfngbp75u-_domainkey-gifbooth-co-CNAME" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "vo3lqx4fdwkk5ipwghlrbifnfngbp75u._domainkey.gifbooth.co"
  type    = "CNAME"
  records = ["vo3lqx4fdwkk5ipwghlrbifnfngbp75u.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "yibczxpctizfstbzlk26eqc4li4xwtax-_domainkey-gifbooth-co-CNAME" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "yibczxpctizfstbzlk26eqc4li4xwtax._domainkey.gifbooth.co"
  type    = "CNAME"
  records = ["yibczxpctizfstbzlk26eqc4li4xwtax.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "gifs-gifbooth-co-A" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "gifs.gifbooth.co"
  type    = "A"

  alias {
    name                   = "d1pdqg40kkxu3t.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "proxy-gifbooth-co-CNAME" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "proxy.gifbooth.co"
  type    = "CNAME"
  records = ["gifbooth-proxy.herokuapp.com"]
  ttl     = "300"
}

resource "aws_route53_record" "r29-gifbooth-co-A" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "r29.gifbooth.co"
  type    = "A"

  alias {
    name                   = "s3-website-us-east-1.amazonaws.com"
    zone_id                = "Z3AQBSTGFYJSTF"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "server-gifbooth-co-A" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "server.gifbooth.co"
  type    = "A"

  alias {
    name                   = "dualstack.awseb-e-9-awsebloa-1si8j8g8872wl-164988714.us-east-1.elb.amazonaws.com"
    zone_id                = "Z35SXDOTRQ7X7K"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "www-gifbooth-co-A" {
  zone_id = "${aws_route53_zone.gifbooth-co-public.zone_id}"
  name    = "www.gifbooth.co"
  type    = "A"

  alias {
    name                   = "gifbooth.co"
    zone_id                = "${aws_route53_zone.gifbooth-co-public.zone_id}"
    evaluate_target_health = false
  }
}
