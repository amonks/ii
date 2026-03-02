package main

import "monks.co/pkg/requireenv"

var placesBrowserAPIKey = requireenv.Lazy("GOOGLE_PLACES_BROWSER_API_KEY")
