package main

import "monks.co/pkg/requireenv"

var lastFmAPIKey = requireenv.Lazy("LASTFM_API_KEY")
