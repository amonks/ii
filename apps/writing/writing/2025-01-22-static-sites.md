---
title: On "Static" Sites
published: false
---

# What is a "static site" anyway?

So-called "static sites" are popular. The term refers to web hosting where your
vendor controls your server code: you just upload HTML files and they handle it.

I'll walk through some of the arguments I've heard in favor of static sites, and
comment on whether they make sense to me.

## "With static sites I don't need a server"

Sure you do. You have a server, and it's probably doing a lot. It just doesn't
feel like that because the server a cloud provider black box that you don't see
or control.

What's your server doing?

- Performing certificate renewals (for what domains? wildcards?)
- Negotiating and terminating TLS (what tls version?)
- Content negotiation: at minimum, it's serving gzipped versions of your files
  based on the ‘Accept-encoding' request header (compressed on the fly or at
  upload time? at what compression level?)
- Setting response headers, like ‘Cache-control' (but to what values?)
- Routing/anycast: your provider has servers in different regions, and responds
  to requests from the one with the shortest line to the requester. (Which
  regions?)
- Distribution/server-side caching: when you upload new files, your provider
  somehow shuttles them to their servers in different regions. (When does each
  region get your new files? How soon after your upload are the new files
  served?)

So: you absolutely have a server. You just don't control its behavior. Each
provider has different bespoke answers to the parenthetical questions above. If
you're lucky, that behavior is at least well-documented

## "OK I do need a server but it's a commodity server"

Kind of? Maybe you don't care about the product differences parenthesized above,
so these services are practically indistinguishable for you. But still: static
site hosting is only provided by a tiny handful of massively scaled cloud
companies (or resellers thereof), not by an undifferentiated mass of local
producers.

In contrast, consider some other server products: general-purpose VMs, colo
space, bare metal, FTP access to a machine with Apache. You can get any of these
things from small businesses in your city. Or you can get them from massive
cloud companies, but either way the offerings are the same. Literally
indistinguishable.

## "Static sites are portable: I can give someone the html files on a usb drive"

OK, first off: you probably can't, actually. Many web features require a "secure
context", which means they don't work if your page is loaded into the browser
statically: they require a server to do work per-request to terminate TLS. So
your thumb drive had better also include some server software. Which is the
second response: you can put server software on a thumb drive too.

## "I don't want traffic data to be collected"

Frankly, I think this argument is actually backwards: even if they don't share
the data with you, I assure you that all static site vendors collect
server/traffic logs.

In fact, the only way to prevent this is to terminate your own TLS (so your
vendor can't see what URLs are being visited) and _not collect traffic logs_.
This is only possible if you control your server code.

## "I don't want to store cleartext user data in in a database"

Neither do I. So don't do that. Controlling your server code doesn't preclude
whatever data storage technique you were planning on: you can still use local
storage or webrtc or the filesystem api or long URLs or whatever. Plus it gives
you some more options: e2e-encrypt user data and make it available (given the
key) across your users' devices.

## "I like that my visitors can see all the code"

This is a good point, actually.

If you care about this for a security reason, e2e encryption (so users don't
_have to_ trust the server) might be helpful.

If you care about this for a pedagogical reason, publishing and linking-to your
server code might help. My one push-back on the pedagogical point is that I
think not having server code often encourages developers to make very complex
frontend code—lots of javascript, minification, frameworks, node modules, crazy
bundle sizes. While this code is _technically_ available to users, it's often
not really tractable to go through and understand. Whereas a server-templated
javascript-light HTML and CSS page might actually be more useful pedagogically,
esepcially given the server code, since it's so much easier to understand.

## "Static file hosting is free"

I wonder who the product is, lol, lmao...

I think this is actually kind of anticompetitive. I think the norm that static
site hosting is free is one of the reasons why the _only_ vendors offering are
hosting are hyperscaled clouds.

But still, I'll concede this point. If you're very budget-constrained, and if
you're serving a _shitload_ of traffic (like, millions of simultaneous
connections) or your content is large (like 10s of gigabytes), you might be in a
space where VM pricing is cost prohibitive, and the "free static site hosting
regardless of scale" norm really does enable your site to operate.

But do remember: you can serve a lot of traffic from a ≤5$/month VM.

## "I can provide an equally good experience without server code"

This can be true! But I think a lot of people underestimate how useful server
code can be.

- Non-dynamic webpages (like a portfolio or blog) can use a server to serve
  right-sized images or to provide search across documents that aren't loaded
  into the browser.
- Single-player document-oriented webapps (where you might use local storage)
  can store e2e-encrypted documents on the server to avoid losing data when a
  user's device breaks.
