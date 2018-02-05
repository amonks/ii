resource "aws_route53_zone" "radblock-xyz-public" {
  name    = "radblock.xyz"
  comment = "Managed by Terraform"

  tags {}
}

resource "aws_route53_record" "radblock-xyz-A" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "radblock.xyz"
  type    = "A"

  alias {
    name                   = "d349pl3yyh2zjc.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "radblock-xyz-MX" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "radblock.xyz"
  type    = "MX"
  records = ["10 inbound-smtp.us-east-1.amazonaws.com."]
  ttl     = "300"
}

resource "aws_route53_record" "radblock-xyz-NS" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "radblock.xyz"
  type    = "NS"
  records = ["ns-1144.awsdns-15.org", "ns-1990.awsdns-56.co.uk", "ns-37.awsdns-04.com", "ns-886.awsdns-46.net"]
  ttl     = "30"
}

resource "aws_route53_record" "radblock-xyz-SOA" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "radblock.xyz"
  type    = "SOA"
  records = ["ns-1990.awsdns-56.co.uk. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400"]
  ttl     = "900"
}

resource "aws_route53_record" "_amazonses-radblock-xyz-TXT" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "_amazonses.radblock.xyz"
  type    = "TXT"
  records = ["HddxiqfGkLcgONdunZkP+rgxAomlGqeRxYocch3GAoY="]
  ttl     = "1800"
}

resource "aws_route53_record" "4klau6rhbc2dx4lq5fifcnrziujq22iw-_domainkey-radblock-xyz-CNAME" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "4klau6rhbc2dx4lq5fifcnrziujq22iw._domainkey.radblock.xyz"
  type    = "CNAME"
  records = ["4klau6rhbc2dx4lq5fifcnrziujq22iw.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "kapxtpmrmqkudo75vusxnmmyjidtp6b2-_domainkey-radblock-xyz-CNAME" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "kapxtpmrmqkudo75vusxnmmyjidtp6b2._domainkey.radblock.xyz"
  type    = "CNAME"
  records = ["kapxtpmrmqkudo75vusxnmmyjidtp6b2.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "ptdgflihnjxt2scng7z7jcmi7f6e4m7m-_domainkey-radblock-xyz-CNAME" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "ptdgflihnjxt2scng7z7jcmi7f6e4m7m._domainkey.radblock.xyz"
  type    = "CNAME"
  records = ["ptdgflihnjxt2scng7z7jcmi7f6e4m7m.dkim.amazonses.com"]
  ttl     = "1800"
}

resource "aws_route53_record" "gifs-radblock-xyz-A" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "gifs.radblock.xyz"
  type    = "A"

  alias {
    name                   = "d3rqnznu9ff0vi.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}

resource "aws_route53_record" "list-radblock-xyz-A" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "list.radblock.xyz"
  type    = "A"

  alias {
    name                   = "d2xydghsy5cfh0.cloudfront.net"
    zone_id                = "Z2FDTNDATAQYW2"
    evaluate_target_health = false
  }
}
