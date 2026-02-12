package main

import "monks.co/pkg/requireenv"

var lastFmAPIKey = requireenv.Require("LASTFM_API_KEY")
