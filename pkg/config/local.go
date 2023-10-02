//go:build !fly
// +build !fly

package config

import "monks.co/pkg/ptr"

var Current = &Configuration{
	StoragePath: "/data",
	Services: []Service{
		{
			Port:     8080,
			Protocol: "http",
			IsAdmin:  false,
		},
		{
			Port:     4433,
			Protocol: "https",
			IsAdmin:  false,
			ACME: &ACME{
				StoragePath: ptr.String("/data/acme"),
				Production:  true,
				Domains: []string{
					"belgianman.com",
					"blgn.mn",
					"monks.co",
					"monks-go.fly.dev",
				},
				Strategies: []ACMEStrategy{
					{
						Strategy:     "alpn",
						ExternalPort: 443,
						InternalPort: 4433,
					},
				},
			},
		},
	},
	Apps: []App{
		{
			Name:     "places",
			Path:     "/places",
			IsPublic: true,
		},
		{
			Name:     "weblog",
			Path:     "/",
			IsPublic: true,
		},
	},
}
