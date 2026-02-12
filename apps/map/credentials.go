package main

import "monks.co/pkg/requireenv"

var placesBrowserAPIKey = requireenv.Require("GOOGLE_PLACES_BROWSER_API_KEY")
