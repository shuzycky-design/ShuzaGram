-- Global, admin-tunable thresholds for the graduated anti-spam restriction
-- system. Singleton row (id pinned to 1) -- the natural generalization of
-- star_gift_dissolve_settings to a feature with no per-entity key. Consumed
-- server-side only (report hook + decay job + RPC gate), never by the client,
-- so a dedicated table is used instead of the AppConfig JSON channel.
CREATE TABLE public.spam_restriction_settings (
  id smallint PRIMARY KEY DEFAULT 1,
  tier1_threshold integer NOT NULL DEFAULT 3,
  tier2_threshold integer NOT NULL DEFAULT 8,
  decay_hours integer NOT NULL DEFAULT 168,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by text NOT NULL DEFAULT '',
  CONSTRAINT spam_restriction_settings_singleton_check CHECK (id = 1),
  CONSTRAINT spam_restriction_settings_thresholds_check CHECK (
    tier1_threshold > 0 AND tier2_threshold > tier1_threshold AND decay_hours > 0
  )
);

INSERT INTO public.spam_restriction_settings (id) VALUES (1);

-- Per-user current restriction state. Kept independent of
-- account_restrictions (account freeze): freeze is a binary, account-wide
-- read-only gate; this is a narrower, three-tier, peer-scoped private
-- messaging gate that must decay on its own clock. Absence of a row means
-- "not restricted" (tier 0) -- the overwhelming majority of users never get
-- a row written here.
CREATE TABLE public.spam_restrictions (
  user_id bigint PRIMARY KEY REFERENCES public.users(id) ON DELETE CASCADE,
  tier smallint NOT NULL DEFAULT 0,
  case_id bigint REFERENCES public.moderation_cases(id) ON DELETE SET NULL,
  distinct_reporter_count integer NOT NULL DEFAULT 0,
  last_report_at timestamptz NOT NULL,
  manual_override boolean NOT NULL DEFAULT false,
  actor text NOT NULL DEFAULT '',
  reason text NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT spam_restrictions_tier_check CHECK (tier IN (0, 1, 2)),
  CONSTRAINT spam_restrictions_version_check CHECK (version > 0)
);

CREATE INDEX spam_restrictions_decay_idx ON public.spam_restrictions (last_report_at)
  WHERE tier > 0 AND NOT manual_override;
