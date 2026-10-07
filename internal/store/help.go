package store

import (
	"context"

	"telesrv/internal/domain"
)

// AppConfigStore 持久化 help.getAppConfig 数据。
type AppConfigStore interface {
	GetAppConfig(ctx context.Context, client string) (domain.AppConfig, bool, error)
	UpsertAppConfig(ctx context.Context, cfg domain.AppConfig) error
}

// CountryStore 持久化 help.getCountriesList 数据。
type CountryStore interface {
	ListCountries(ctx context.Context, langCode string) (domain.CountriesList, error)
	UpsertCountries(ctx context.Context, countries []domain.Country) error
	// DeleteCountry removes one country (and its country_codes rows, via
	// ON DELETE CASCADE) by ISO2. Used by the admin "custom country codes"
	// tool to retract a self-added entry -- the ~235-country catalog reseed
	// on every startup (see cmd/telesrv/main.go) never deletes anything, so
	// this is the only way to remove one.
	DeleteCountry(ctx context.Context, iso2 string) error
}
