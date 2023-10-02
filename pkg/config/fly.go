//go:build fly
// +build fly

package config

var Config = &Config{
	StoragePath: "/data",
	Services: []Service{
		{
			Port:     1337,
			Protocol: "http",
			IsAdmin:  false,
		},
		{
			Port:     433,
			Protocol: "https",
			IsAdmin:  false,
			ACME: &ACME{
				Production: true,
				Domains: []string{
					"brigid.ss.cx",
				},
				Strategies: []ACMEStrategy{
					{
						Strategy: "dns",
					},
				},
			},
		},
	},
	Apps: []App{
		{
			Name:     "places",
			Path:     "/places",
			IsPublic: false,
		},
		{
			Name:     "weblog",
			Path:     "/",
			IsPublic: false,
		},
		{
			Name:     "ping",
			Path:     "/ping",
			IsPublic: false,
		},
	},
}
