- always use red-green tdd; we don't have good test coverage at the moment but we would like to. taking time to write tests is good.
- we manage a bunch of apps in ./apps/xyz
- apps run on different hosts, but all share a tailnet, and we access them via apps/proxy
- eg: https://monks.co/dogs -- the proxy handles that request and forwards it to the dogs app
- deploy apps by running 'fly deploy -c apps/$app/fly.toml' from .
- never run 'go build' -- it polutes the working directory with binary artifacts. use 'go test', which implies 'go build' but goes to tmp
- routing is configured through tailscale capability grants. As I write this, the configuration is as follows:

<routing>

    	// The whole internet can use public services
    	{
    		"src": ["autogroup:danger-all"],
    		"dst": ["tag:monks-co"],
    		"app": {
    			"monks.co/cap/public": [
    				// fly
    				{"path": "dogs", "backend": "monks-dogs-fly-ord"},
    				{"path": "homepage", "backend": "monks-homepage-fly-ord"},
    				{"path": "map", "backend": "monks-map-fly-ord"},
    				{"path": "writing", "backend": "monks-writing-fly-ord"},
    			],
    		},
    	},
    	// Users can use local services
    	{
    		"src": ["autogroup:member"],
    		"dst": ["tag:monks-co"],
    		"app": {
    			"monks.co/cap/public": [
    				// thor
    				{"path": "air", "backend": "monks-air-thor"},
    				{"path": "movies", "backend": "monks-movies-thor"},
    			],
    		},
    	},
    	// Services can use service-facing services
    	{
    		"src": ["tag:service"],
    		"dst": ["tag:monks-co"],
    		"app": {
    			"monks.co/cap/public": [
    				// fly
    				{"path": "traffic", "backend": "monks-traffic-fly-ord"},
    				{"path": "logs", "backend": "monks-logs-fly-ord"},

    				// brigid
    				{"path": "sms", "backend": "monks-sms-brigid"},
    			],
    		},
    	},
    	// ajm@passkey can use private services
    	{
    		"src": ["ajm@passkey"],
    		"dst": ["tag:monks-co"],
    		"app": {
    			"monks.co/cap/public": [
    				// brigid
    				{"path": "calendar", "backend": "monks-calendar-brigid"},
    				{"path": "directory", "backend": "monks-directory-brigid"},
    				{"path": "golink", "backend": "monks-golink-brigid"},
    				{"path": "ping", "backend": "monks-ping-brigid"},
    				{"path": "scrobbles", "backend": "monks-scrobbles-brigid"},
    				{"path": "logs", "backend": "monks-logs-fly-ord"},
    				{"path": "youtube", "backend": "monks-youtube-brigid"},

    				// thor
    				{"path": "reddit", "backend": "monks-reddit-thor"},
    			],
    		},
    	},

</routing>
