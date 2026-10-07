package postgres

import (
	"context"
	"testing"

	"telesrv/internal/domain"
)

// TestHelpStoreCustomCountryCodeLifecyclePostgres locks the "custom country
// codes" admin tool end to end against real Postgres: a self-hosted ISO2
// longer than the original varchar(2) (migration 0207_widen_country_iso2)
// can be upserted with multiple dialing codes, is visible through
// ListCountries exactly as written, and DeleteCountry removes it -- and,
// via the countries->country_codes ON DELETE CASCADE, its codes too, not
// just the country row.
func TestHelpStoreCustomCountryCodeLifecyclePostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := NewHelpStore(pool)

	const iso2 = "SGT"
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM countries WHERE iso2 = $1`, iso2)
	})

	err := store.UpsertCountries(ctx, []domain.Country{{
		ISO2: iso2, DefaultName: "ShuzaGram Test Range", Hidden: false,
		CountryCodes: []domain.CountryCode{
			{CountryCode: "999", Patterns: []string{"XXX XX XXX"}},
			{CountryCode: "998", Prefixes: []string{"1", "2"}},
		},
	}})
	if err != nil {
		t.Fatalf("upsert custom country: %v", err)
	}

	list, err := store.ListCountries(ctx, "")
	if err != nil {
		t.Fatalf("list countries: %v", err)
	}
	var found domain.Country
	ok := false
	for _, c := range list.Countries {
		if c.ISO2 == iso2 {
			found, ok = c, true
			break
		}
	}
	if !ok {
		t.Fatalf("upserted country %q not found in catalog of %d", iso2, len(list.Countries))
	}
	if found.DefaultName != "ShuzaGram Test Range" || len(found.CountryCodes) != 2 {
		t.Fatalf("round-tripped country = %+v", found)
	}

	if err := store.DeleteCountry(ctx, iso2); err != nil {
		t.Fatalf("delete country: %v", err)
	}

	var codeCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM country_codes WHERE iso2 = $1`, iso2).Scan(&codeCount); err != nil {
		t.Fatalf("count orphaned country codes: %v", err)
	}
	if codeCount != 0 {
		t.Fatalf("country_codes rows survived DeleteCountry: %d, want 0 (cascade)", codeCount)
	}

	list, err = store.ListCountries(ctx, "")
	if err != nil {
		t.Fatalf("list countries after delete: %v", err)
	}
	for _, c := range list.Countries {
		if c.ISO2 == iso2 {
			t.Fatalf("deleted country %q still present", iso2)
		}
	}
}
