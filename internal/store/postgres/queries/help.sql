-- name: GetAppConfig :one
SELECT client, hash, config_json::text AS config_json
FROM app_configs
WHERE client = $1;

-- name: UpsertAppConfig :exec
INSERT INTO app_configs (client, hash, config_json)
VALUES ($1, $2, sqlc.arg(config_json)::jsonb)
ON CONFLICT (client) DO UPDATE SET
  hash = EXCLUDED.hash,
  config_json = EXCLUDED.config_json,
  updated_at = now();

-- name: ListCountries :many
SELECT
  c.iso2,
  c.default_name,
  c.name,
  c.hidden,
  cc.country_code,
  cc.prefixes,
  cc.patterns
FROM countries c
JOIN country_codes cc ON cc.iso2 = c.iso2
ORDER BY c.order_index, c.iso2, cc.order_index, cc.country_code;

-- name: UpsertCountry :exec
INSERT INTO countries (iso2, default_name, name, hidden, order_index)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (iso2) DO UPDATE SET
  default_name = EXCLUDED.default_name,
  name = EXCLUDED.name,
  hidden = EXCLUDED.hidden,
  order_index = EXCLUDED.order_index,
  updated_at = now();

-- name: UpsertCountryCode :exec
INSERT INTO country_codes (iso2, country_code, prefixes, patterns, order_index)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (iso2, country_code) DO UPDATE SET
  prefixes = EXCLUDED.prefixes,
  patterns = EXCLUDED.patterns,
  order_index = EXCLUDED.order_index;

-- name: DeleteCountry :exec
DELETE FROM countries WHERE iso2 = $1;
