package googlemaps

import "monks.co/pkg/requireenv"

var placesBackendAPIKey = requireenv.Lazy("GOOGLE_PLACES_BACKEND_API_KEY")
