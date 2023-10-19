package weblog

import "github.com/go-ap/activitypub"

func getActor() ([]byte, error) {
	actor := &activitypub.Actor{
		ID:   "https://monks.co",
		Type: activitypub.PersonType,

		Inbox:  activitypub.LinkNew("https://monks.co/inbox", activitypub.OrderedCollectionType),
		Outbox: activitypub.LinkNew("https://monks.co/outbox", activitypub.OrderedCollectionType),

		Following: activitypub.LinkNew("https://monks.co/following", activitypub.OrderedCollectionType),
		Followers: activitypub.LinkNew("https://monks.co/followers", activitypub.OrderedCollectionType),

		PreferredUsername: activitypub.NaturalLanguageValues{activitypub.LangRefValueNew("en-us", "amonks")},

		PublicKey: activitypub.PublicKey{
			ID:           "https://monks.co#public-key",
			Owner:        "https://monks.co",
			PublicKeyPem: "",
		},
	}

	return actor.MarshalJSON()
}

func getWebfinger() []byte {
	return []byte(`
	{
		"subject": "acct:a@monks.co",

		"links": [
			{
				"rel": "canonical_uri",
				"type": "text/html",
				"href": "https://monks.co"
			},
			{
				"rel": "http://webfinger.net/rel/avatar",
				"href": "http://monks.co/headshot.jpg"
			},
			{
				"rel": "self",
				"type": "application/activity+json",
				"href": "https://monks.co"
			},
			{
				"rel": "inbox",
				"type": "application/activity+json",
				"href": "https://monks.co/inbox"
			}
		]
	}
	`)
}
