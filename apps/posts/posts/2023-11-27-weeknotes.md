---
title: Weeknotes 11/26/2023
draft: false
---

I have a few projects in flight this week.

## Ballotine

My fixation right now is ballotine. That's when you debone, stuff, truss, and
roast a whole bird.

It seems like a great Thanksgiving recipe: there aren't any bones to complicate
table-side carving or turn the meat red! Plus it accomodates stuffing and looks
whole enough to be Thanksgivingy. It doesn't solve the undercooked breasts
problem, but I guess that means the overcooked breasts have a nice veneer of
old-school French provenance.

But it's also a terrible Thanksgiving recipe: it's pretty technical, and
Thanksgiving is an important meal to not ruin! It's a bad time to whip out an
advanced technique that you aren't any good at.

This year, though, I don't have anyone coming for Thanksgiving, so it's ok to
ruin dinner! So my mission is to get good at ballotine. I did a chicken on
Friday, a capon on Sunday, and I'm working towards something for a dinner party
on Wednesday.

I'm getting better at the deboning. On the chicken, I did the
gradually-scrape-meat-from-bone technique, and it was awful. It worked in the
end, but it took forever and I constantly felt like I was doing it wrong. On
the capon, I used Jacques Pepin's technique where you peel the flesh from the
cage with your hands. I still felt like I was doing it wrong, but it only took
5 or 6 minutes, so that's a win.

This was my first capon. Apparently if you castrate a rooster, they lose
interest in sex and exercise and gain a passion for lounging and drinking
cream. This gives them more intramuscular fat. Capons are harvested at all
sizes, but the conventional ones at my local supermarket are about 10 pounds,
which is a few pounds larger than the chickens. Verdict: definitely juicier,
but not a big flavor difference. I think swapping a conventional for a heritage
bird, or a young for an old one has more of an impact. I wouldn't buy a larger
bird than I wanted just to get a capon, but if there were both capons and
chickens in my desired size, I'd probably go for the capon.

I like the presentation, and I definitely want to keep practicing deboning, but
I'm not thrilled with the overcooked breasts. I see that some folks get around
this by doing a ballotine-like skin-wrapped-log of -just- the breasts, then
perhaps incorporate the (differently cooked) legs into the stuffing.
Separately, I wonder if I could carefully remove the meat from the skin, sous
vide the breasts and legs separately, recombine them into the log, put the skin
back on (meat glue?), and crisp with hot oil. That might be next.

## Movies

I have a program for organizing my movie library and displaying a Netflix-like
browsing interface. The program has accreted functionality over time: first, I
just dumped an `ls` of my movies folder into sqlite, because that is Always A
Good Thing To Do. A while later, I wrote a thing to try to fetch metadadta from
TMDB. The UI came next. Etc.

This week, I added two integrations I'm pretty happy with. First, a thing to
pull ratings from Metacritic, enabling a sort-by-rating. Second, a thing to
pull my reviews from Letterboxd, enabling a show-only-unwatched and a
sort-by-my-personal-review-score.

As a bonus, I added a thingy to show my last `n` reviews on my homepage. This
is a bit of a hack. All of my little personal services are compiled into the
same binary, and I run it with different configuration on various computers,
including on fly.io for public-facing servcies. The movie library service runs
on my home server, so its data lives in an sqlite file at home. But my homepage
runs on a rented machine from fly.io, so how does it get the movie data?
Answer: a clunky `expect` script that copies the db file over and restarts the
fly box. I am very excited for the day that Hermit, my syncing embedded
database, is featureful enough to support use cases like this.

## Run

I picked work back up on an incremental-search feature for [my little task
runner](https://github.com/amonks/run). I started this a few months ago, but
realized it'd require writing my own line-aware wrapping display thingy, and
kind of lost interest.

My personal services binary (see above) uses the task runner, and I noticed
that if I leave it running for a long time, the log display gets gunked up and
stops working. Profiling says that this is because of wrapping in the
bubbletea-default viewer component I'm using: each frame rewraps the whole log,
so it's O(log length) rather than O(screen size).

So now I have to rewrite the log viewer anyway, so I may as well take this
opportunity to add incremental search. It works, kinda! WIP in [a branch
here](https://github.com/amonks/run/pull/75).
