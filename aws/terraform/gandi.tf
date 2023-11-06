resource "gandi_nameservers" "gandi_domain_andrewmonks_com" {
  domain      = "andrewmonks.com"
  nameservers = aws_route53_zone.andrewmonks-com.name_servers
}

resource "gandi_nameservers" "gandi_domain_andrewmonks_net" {
  domain      = "andrewmonks.net"
  nameservers = aws_route53_zone.andrewmonks-net.name_servers
}

resource "gandi_nameservers" "gandi_domain_andrewmonks_org" {
  domain      = "andrewmonks.org"
  nameservers = aws_route53_zone.andrewmonks-org.name_servers
}

resource "gandi_nameservers" "gandi_domain_belgianman_com" {
  domain      = "belgianman.com"
  nameservers = aws_route53_zone.belgianman-com.name_servers
}

resource "gandi_nameservers" "gandi_domain_blgn_mn" {
  domain      = "blgn.mn"
  nameservers = aws_route53_zone.blgn-mn.name_servers
}

resource "gandi_nameservers" "gandi_domain_docrimes_com" {
  domain      = "docrimes.com"
  nameservers = aws_route53_zone.docrimes-com.name_servers
}

resource "gandi_nameservers" "gandi_domain_fmail_email" {
  domain      = "fmail.email"
  nameservers = aws_route53_zone.fmail-email.name_servers
}

resource "gandi_nameservers" "gandi_domain_lyrics_gy" {
  domain      = "lyrics.gy"
  nameservers = aws_route53_zone.lyrics-gy.name_servers
}

resource "gandi_nameservers" "gandi_domain_needsyourhelp_org" {
  domain      = "needsyourhelp.org"
  nameservers = aws_route53_zone.needsyourhelp-org.name_servers
}

resource "gandi_nameservers" "gandi_domain_popefucker_com" {
  domain      = "popefucker.com"
  nameservers = aws_route53_zone.popefucker-com.name_servers
}

resource "gandi_nameservers" "gandi_domain_ss_cx" {
  domain      = "ss.cx"
  nameservers = aws_route53_zone.ss-cx.name_servers
}

resource "gandi_nameservers" "gandi_domain_monks_co" {
  domain      = "monks.co"
  nameservers = aws_route53_zone.monks-co.name_servers
}
