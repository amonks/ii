package config

import "monks.co/pkg/service"

type Configuration struct {
	StoragePath string
	Services    []Service
	Apps        []App
}

type Service struct {
	Port     int
	Protocol string
	IsAdmin  bool
	ACME     *ACME
}

type ACME struct {
	StoragePath *string
	Strategies  []ACMEStrategy
	Domains     []string
	Production  bool
}

type ACMEStrategy struct {
	Strategy     string
	ExternalPort int
	InternalPort int
}

type App struct {
	Service  service.Service
	Name     string
	Path     string
	IsPublic bool
	Hosts    []string
}
