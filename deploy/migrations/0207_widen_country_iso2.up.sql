-- The admin "custom country codes" tool (internal/admin.Service.UpsertCountry)
-- accepts a 2-4 letter ISO2 on purpose -- a genuinely custom entry needs
-- something that can never collide with a real ISO 3166-1 alpha-2 code
-- (always exactly 2 letters), and a 3-4 letter tag is the simplest way to
-- guarantee that. The original schema's countries.iso2/country_codes.iso2
-- were both varchar(2), matching only the real catalog, so anything longer
-- failed with a raw Postgres "value too long" error instead of the
-- application's own validation message. Widen both to varchar(4) (the
-- FK between them requires matching-enough types) to actually allow what
-- the application layer already promises.
ALTER TABLE public.countries ALTER COLUMN iso2 TYPE character varying(4);
ALTER TABLE public.country_codes ALTER COLUMN iso2 TYPE character varying(4);
