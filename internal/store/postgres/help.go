package postgres

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"

	"github.com/jackc/pgx/v5"

	"telesrv/internal/domain"
	"telesrv/internal/store/postgres/sqlcgen"
)

// HelpStore 用 PostgreSQL 实现 store.AppConfigStore 和 store.CountryStore。
type HelpStore struct {
	q *sqlcgen.Queries
}

// NewHelpStore 基于 pgx 连接池（或事务）创建 HelpStore。
func NewHelpStore(db sqlcgen.DBTX) *HelpStore {
	return &HelpStore{q: sqlcgen.New(db)}
}

func (s *HelpStore) GetAppConfig(ctx context.Context, client string) (domain.AppConfig, bool, error) {
	row, err := s.q.GetAppConfig(ctx, client)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AppConfig{Client: client, JSON: []byte("{}")}, false, nil
		}
		return domain.AppConfig{}, false, fmt.Errorf("get app config: %w", err)
	}
	return domain.AppConfig{
		Client: row.Client,
		Hash:   int(row.Hash),
		JSON:   []byte(row.ConfigJson),
	}, true, nil
}

func (s *HelpStore) UpsertAppConfig(ctx context.Context, cfg domain.AppConfig) error {
	if len(cfg.JSON) == 0 {
		cfg.JSON = []byte("{}")
	}
	if err := s.q.UpsertAppConfig(ctx, sqlcgen.UpsertAppConfigParams{
		Client:     cfg.Client,
		Hash:       int32(cfg.Hash),
		ConfigJson: cfg.JSON,
	}); err != nil {
		return fmt.Errorf("upsert app config: %w", err)
	}
	return nil
}

func (s *HelpStore) ListCountries(ctx context.Context, _ string) (domain.CountriesList, error) {
	rows, err := s.q.ListCountries(ctx)
	if err != nil {
		return domain.CountriesList{}, fmt.Errorf("list countries: %w", err)
	}
	byISO := make(map[string]int)
	out := domain.CountriesList{}
	for _, row := range rows {
		idx, ok := byISO[row.Iso2]
		if !ok {
			idx = len(out.Countries)
			byISO[row.Iso2] = idx
			out.Countries = append(out.Countries, domain.Country{
				ISO2:        row.Iso2,
				DefaultName: row.DefaultName,
				Name:        row.Name,
				Hidden:      row.Hidden,
			})
		}
		out.Countries[idx].CountryCodes = append(out.Countries[idx].CountryCodes, domain.CountryCode{
			CountryCode: row.CountryCode,
			Prefixes:    append([]string(nil), row.Prefixes...),
			Patterns:    append([]string(nil), row.Patterns...),
		})
	}
	out.Hash = countriesHash(out.Countries)
	return out, nil
}

func (s *HelpStore) UpsertCountries(ctx context.Context, countries []domain.Country) error {
	for i, country := range countries {
		if err := s.q.UpsertCountry(ctx, sqlcgen.UpsertCountryParams{
			Iso2:        country.ISO2,
			DefaultName: country.DefaultName,
			Name:        country.Name,
			Hidden:      country.Hidden,
			OrderIndex:  int32(i + 1),
		}); err != nil {
			return fmt.Errorf("upsert country %q: %w", country.ISO2, err)
		}
		for j, code := range country.CountryCodes {
			// prefixes/patterns 列 NOT NULL:无前缀/格式的国家(如 AD)catalog 里为 nil,
			// 须规整为空数组,否则 pgx 写 NULL 触发 23502。
			prefixes := code.Prefixes
			if prefixes == nil {
				prefixes = []string{}
			}
			patterns := code.Patterns
			if patterns == nil {
				patterns = []string{}
			}
			if err := s.q.UpsertCountryCode(ctx, sqlcgen.UpsertCountryCodeParams{
				Iso2:        country.ISO2,
				CountryCode: code.CountryCode,
				Prefixes:    prefixes,
				Patterns:    patterns,
				OrderIndex:  int32(j + 1),
			}); err != nil {
				return fmt.Errorf("upsert country code %q/%q: %w", country.ISO2, code.CountryCode, err)
			}
		}
	}
	return nil
}

func (s *HelpStore) DeleteCountry(ctx context.Context, iso2 string) error {
	if err := s.q.DeleteCountry(ctx, iso2); err != nil {
		return fmt.Errorf("delete country %q: %w", iso2, err)
	}
	return nil
}

func countriesHash(countries []domain.Country) int {
	if len(countries) == 0 {
		return 0
	}
	h := fnv.New32a()
	var buf [4]byte
	for _, country := range countries {
		_, _ = h.Write([]byte(country.ISO2))
		_, _ = h.Write([]byte(country.DefaultName))
		_, _ = h.Write([]byte(country.Name))
		if country.Hidden {
			buf[0] = 1
		} else {
			buf[0] = 0
		}
		_, _ = h.Write(buf[:1])
		for _, code := range country.CountryCodes {
			_, _ = h.Write([]byte(code.CountryCode))
			binary.LittleEndian.PutUint32(buf[:], uint32(len(code.Prefixes)))
			_, _ = h.Write(buf[:])
			for _, prefix := range code.Prefixes {
				_, _ = h.Write([]byte(prefix))
			}
			binary.LittleEndian.PutUint32(buf[:], uint32(len(code.Patterns)))
			_, _ = h.Write(buf[:])
			for _, pattern := range code.Patterns {
				_, _ = h.Write([]byte(pattern))
			}
		}
	}
	return int(h.Sum32())
}
