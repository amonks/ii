resource "aws_route53_zone" "radblock-xyz-public" {
  name = "radblock.xyz"

  tags = {}
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

resource "aws_route53_record" "_amazonses-radblock-xyz-TXT" {
  zone_id = "${aws_route53_zone.radblock-xyz-public.zone_id}"
  name    = "_amazonses.radblock.xyz"
  type    = "TXT"
  records = ["HddxiqfGkLcgONdunZkP+rgxAomlGqeRxYocch3GAoY="]
  ttl     = "1800"
}

resource "aws_route53_record" "a-4klau6rhbc2dx4lq5fifcnrziujq22iw-_domainkey-radblock-xyz-CNAME" {
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
