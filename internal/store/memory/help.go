package memory

import (
	"context"
	"sync"
	"telesrv/internal/domain"
)

// HelpStore 是 store.AppConfigStore 和 store.CountryStore 的内存实现。
type HelpStore struct {
	mu        sync.RWMutex
	appConfig map[string]domain.AppConfig
	countries domain.CountriesList
}

// NewHelpStore 创建内存 HelpStore。
func NewHelpStore() *HelpStore {
	return &HelpStore{appConfig: make(map[string]domain.AppConfig)}
}

func (s *HelpStore) GetAppConfig(_ context.Context, client string) (domain.AppConfig, bool, error) {
	s.mu.RLock()
	cfg, ok := s.appConfig[client]
	s.mu.RUnlock()
	cfg.JSON = append([]byte(nil), cfg.JSON...)
	return cfg, ok, nil
}

func (s *HelpStore) UpsertAppConfig(_ context.Context, cfg domain.AppConfig) error {
	cfg.JSON = append([]byte(nil), cfg.JSON...)
	s.mu.Lock()
	s.appConfig[cfg.Client] = cfg
	s.mu.Unlock()
	return nil
}

func (s *HelpStore) ListCountries(_ context.Context, _ string) (domain.CountriesList, error) {
	s.mu.RLock()
	list := s.countries
	s.mu.RUnlock()
	list.Countries = append([]domain.Country(nil), list.Countries...)
	for i := range list.Countries {
		list.Countries[i].CountryCodes = append([]domain.CountryCode(nil), list.Countries[i].CountryCodes...)
	}
	return list, nil
}

func (s *HelpStore) UpsertCountries(_ context.Context, countries []domain.Country) error {
	list := domain.CountriesList{Hash: 1, Countries: append([]domain.Country(nil), countries...)}
	for i := range list.Countries {
		list.Countries[i].CountryCodes = append([]domain.CountryCode(nil), list.Countries[i].CountryCodes...)
	}
	s.mu.Lock()
	s.countries = list
	s.mu.Unlock()
	return nil
}

func (s *HelpStore) DeleteCountry(_ context.Context, iso2 string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.countries.Countries[:0:0]
	for _, c := range s.countries.Countries {
		if c.ISO2 != iso2 {
			out = append(out, c)
		}
	}
	s.countries.Countries = out
	return nil
}
