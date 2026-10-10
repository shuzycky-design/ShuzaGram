package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"telesrv/internal/domain"
	"telesrv/internal/store/postgres/sqlcgen"
)

// SpamRestrictionStore persists per-user graduated anti-spam restriction
// state and the global admin-tunable thresholds driving it. Kept separate
// from AdminStore's account-freeze methods: this is an independent,
// narrower, peer-scoped gate with its own decay clock (see
// internal/rpc/spam_restriction_gate.go and spam_restriction_worker.go).
type SpamRestrictionStore struct {
	db sqlcgen.DBTX
}

func NewSpamRestrictionStore(db sqlcgen.DBTX) *SpamRestrictionStore {
	return &SpamRestrictionStore{db: db}
}

func (s *SpamRestrictionStore) SpamRestriction(ctx context.Context, userID int64) (domain.SpamRestriction, bool, error) {
	if s == nil || s.db == nil || userID <= 0 {
		return domain.SpamRestriction{}, false, nil
	}
	row := s.db.QueryRow(ctx, `
SELECT user_id, tier, case_id, distinct_reporter_count, last_report_at,
       manual_override, actor, reason, version, updated_at
FROM spam_restrictions
WHERE user_id = $1`, userID)
	r, err := scanSpamRestriction(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SpamRestriction{}, false, nil
		}
		return domain.SpamRestriction{}, false, fmt.Errorf("get spam restriction: %w", err)
	}
	return r, true, nil
}

// SetSpamRestriction is the admin manual-override write path: it always sets
// ManualOverride so neither the moderation report hook nor the decay sweep
// touches the row again until an admin explicitly clears the override (by
// writing tier=0 with ManualOverride=false, or any tier with
// ManualOverride=false to hand control back to the automatic system).
func (s *SpamRestrictionStore) SetSpamRestriction(ctx context.Context, r domain.SpamRestriction) (domain.SpamRestriction, error) {
	if s == nil || s.db == nil || r.UserID <= 0 {
		return domain.SpamRestriction{}, fmt.Errorf("valid user is required")
	}
	lastReportAt := r.LastReportAt
	if lastReportAt.IsZero() {
		lastReportAt = time.Now().UTC()
	}
	row := s.db.QueryRow(ctx, `
INSERT INTO spam_restrictions (
	user_id, tier, case_id, distinct_reporter_count, last_report_at,
	manual_override, actor, reason, version, updated_at
)
VALUES ($1,$2,NULLIF($3,0),$4,$5,$6,$7,$8,1,now())
ON CONFLICT (user_id) DO UPDATE SET
	tier = EXCLUDED.tier,
	case_id = EXCLUDED.case_id,
	distinct_reporter_count = EXCLUDED.distinct_reporter_count,
	last_report_at = EXCLUDED.last_report_at,
	manual_override = EXCLUDED.manual_override,
	actor = EXCLUDED.actor,
	reason = EXCLUDED.reason,
	version = spam_restrictions.version + 1,
	updated_at = now()
RETURNING user_id, tier, case_id, distinct_reporter_count, last_report_at,
          manual_override, actor, reason, version, updated_at`,
		r.UserID, int16(r.Tier), r.CaseID, r.DistinctReporterCount, lastReportAt,
		r.ManualOverride, r.Actor, r.Reason,
	)
	out, err := scanSpamRestriction(row)
	if err != nil {
		return domain.SpamRestriction{}, fmt.Errorf("set spam restriction: %w", err)
	}
	return out, nil
}

// SweepDueSpamRestrictionDecay fully clears (tier -> 0) any non-manual,
// currently-restricted row whose last_report_at is older than the
// admin-configured decay window, in bounded batches. It returns the number
// of rows cleared.
func (s *SpamRestrictionStore) SweepDueSpamRestrictionDecay(ctx context.Context, now time.Time, limit int) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	if limit <= 0 {
		limit = 500
	}
	tag, err := s.db.Exec(ctx, `
UPDATE spam_restrictions
SET tier = 0, version = version + 1, updated_at = $1::timestamptz
WHERE user_id IN (
	SELECT sr.user_id
	FROM spam_restrictions sr, spam_restriction_settings cfg
	WHERE cfg.id = 1
	  AND sr.tier > 0
	  AND NOT sr.manual_override
	  AND sr.last_report_at < $1::timestamptz - (cfg.decay_hours * interval '1 hour')
	ORDER BY sr.last_report_at
	LIMIT $2
	FOR UPDATE SKIP LOCKED
)`, now, limit)
	if err != nil {
		return 0, fmt.Errorf("sweep spam restriction decay: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ClaimDueSpamRestrictionNotifications atomically claims and marks-notified
// a batch of rows whose current tier hasn't been messaged to the user yet
// (tier <> notified_tier), returning the (user_id, tier) pairs @spambot
// should message. This is deliberately a single statement, not a
// claim-then-complete lease like account freeze's notification queue: if the
// subsequent SendPrivateText fails, the notification is simply lost rather
// than retried -- acceptable here because @spambot is also reachable
// on-demand (the user can always message it directly to see current
// status), unlike freeze which has no such fallback.
func (s *SpamRestrictionStore) ClaimDueSpamRestrictionNotifications(ctx context.Context, limit int) ([]domain.SpamRestrictionNotification, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(ctx, `
UPDATE spam_restrictions
SET notified_tier = tier
WHERE user_id IN (
	SELECT user_id FROM spam_restrictions
	WHERE tier <> notified_tier
	ORDER BY updated_at
	LIMIT $1
	FOR UPDATE SKIP LOCKED
)
RETURNING user_id, tier`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim spam restriction notifications: %w", err)
	}
	defer rows.Close()
	var out []domain.SpamRestrictionNotification
	for rows.Next() {
		var n domain.SpamRestrictionNotification
		var tier int16
		if err := rows.Scan(&n.UserID, &tier); err != nil {
			return nil, fmt.Errorf("scan spam restriction notification: %w", err)
		}
		n.Tier = domain.SpamRestrictionTier(tier)
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spam restriction notifications: %w", err)
	}
	return out, nil
}

func (s *SpamRestrictionStore) SpamRestrictionSettings(ctx context.Context) (domain.SpamRestrictionSettings, error) {
	if s == nil || s.db == nil {
		return domain.SpamRestrictionSettings{}, fmt.Errorf("spam restriction store is not configured")
	}
	var out domain.SpamRestrictionSettings
	err := s.db.QueryRow(ctx, `
SELECT tier1_threshold, tier2_threshold, decay_hours, updated_at, updated_by
FROM spam_restriction_settings WHERE id = 1`).Scan(
		&out.Tier1Threshold, &out.Tier2Threshold, &out.DecayHours, &out.UpdatedAt, &out.UpdatedBy,
	)
	if err != nil {
		return domain.SpamRestrictionSettings{}, fmt.Errorf("get spam restriction settings: %w", err)
	}
	return out, nil
}

func (s *SpamRestrictionStore) SetSpamRestrictionSettings(ctx context.Context, settings domain.SpamRestrictionSettings, actor string) (domain.SpamRestrictionSettings, error) {
	if s == nil || s.db == nil {
		return domain.SpamRestrictionSettings{}, fmt.Errorf("spam restriction store is not configured")
	}
	if settings.Tier1Threshold <= 0 || settings.Tier2Threshold <= settings.Tier1Threshold || settings.DecayHours <= 0 {
		return domain.SpamRestrictionSettings{}, fmt.Errorf("invalid spam restriction settings")
	}
	var out domain.SpamRestrictionSettings
	err := s.db.QueryRow(ctx, `
UPDATE spam_restriction_settings
SET tier1_threshold = $1, tier2_threshold = $2, decay_hours = $3,
    updated_by = $4, updated_at = now()
WHERE id = 1
RETURNING tier1_threshold, tier2_threshold, decay_hours, updated_at, updated_by`,
		settings.Tier1Threshold, settings.Tier2Threshold, settings.DecayHours, actor,
	).Scan(&out.Tier1Threshold, &out.Tier2Threshold, &out.DecayHours, &out.UpdatedAt, &out.UpdatedBy)
	if err != nil {
		return domain.SpamRestrictionSettings{}, fmt.Errorf("set spam restriction settings: %w", err)
	}
	return out, nil
}

type spamRestrictionScanner interface {
	Scan(dest ...any) error
}

func scanSpamRestriction(row spamRestrictionScanner) (domain.SpamRestriction, error) {
	var (
		out    domain.SpamRestriction
		tier   int16
		caseID *int64
	)
	if err := row.Scan(
		&out.UserID, &tier, &caseID, &out.DistinctReporterCount, &out.LastReportAt,
		&out.ManualOverride, &out.Actor, &out.Reason, &out.Version, &out.UpdatedAt,
	); err != nil {
		return domain.SpamRestriction{}, err
	}
	out.Tier = domain.SpamRestrictionTier(tier)
	if caseID != nil {
		out.CaseID = *caseID
	}
	return out, nil
}
