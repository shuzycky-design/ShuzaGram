-- Built-in @spambot: the user-facing side of the graduated anti-spam
-- restriction system (see 0218_spam_restrictions). Message it any time to
-- see your current restriction tier; it also proactively messages a user
-- whenever their tier changes (see notified_tier below).
--
-- Seeded here rather than lazily on first message, same reasoning as
-- @verifybot (migration 0153): occupies the username from the moment the
-- schema is current, before any ordinary user could claim it.
--
-- access_hash is double-written with domain.SpamBotAccessHash; the two must
-- never drift, exactly as for the other service bots.

INSERT INTO public.users (
    id, access_hash, phone, first_name, last_name, username, country_code,
    created_at, updated_at, verified, support, about, last_seen_at,
    default_history_ttl_period, is_bot, bot_info_version, premium_expires_at,
    emoji_status_document_id, emoji_status_until, color_set, color,
    color_background_emoji_id, profile_color_set, profile_color,
    profile_color_background_emoji_id
) VALUES (
    1250000021, 8562983040105300686, '', 'Spam Bot', '', 'spambot', '',
    now(), now(), true, false,
    'Checks whether your account currently has an anti-spam restriction, and tells you when that changes.',
    0, 0, true, 1, NULL, 0, 0, false, 0, 0, false, 0, 0
)
ON CONFLICT (id) DO UPDATE SET
    access_hash = EXCLUDED.access_hash,
    phone = EXCLUDED.phone,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    username = EXCLUDED.username,
    verified = EXCLUDED.verified,
    support = EXCLUDED.support,
    about = EXCLUDED.about,
    is_bot = EXCLUDED.is_bot,
    bot_info_version = GREATEST(public.users.bot_info_version, EXCLUDED.bot_info_version),
    updated_at = now();

INSERT INTO public.bots (
    bot_user_id, owner_user_id, token_secret, description, commands,
    bot_chat_history, bot_nochats, inline_placeholder, created_at, updated_at,
    menu_button_type, menu_button_text, menu_button_url, bot_inline_geo
) VALUES (
    1250000021, 1250000021, '',
    'Checks whether your account currently has an anti-spam restriction (reports from other users), and tells you when that changes.',
    '[
        {"command": "start", "description": "check your current restriction status"},
        {"command": "status", "description": "check your current restriction status"},
        {"command": "help", "description": "show help"}
    ]'::jsonb,
    false, true, '', now(), now(), 0, '', '', false
)
ON CONFLICT (bot_user_id) DO UPDATE SET
    owner_user_id = EXCLUDED.owner_user_id,
    token_secret = EXCLUDED.token_secret,
    description = EXCLUDED.description,
    commands = EXCLUDED.commands,
    bot_chat_history = EXCLUDED.bot_chat_history,
    bot_nochats = EXCLUDED.bot_nochats,
    inline_placeholder = EXCLUDED.inline_placeholder,
    menu_button_type = EXCLUDED.menu_button_type,
    menu_button_text = EXCLUDED.menu_button_text,
    menu_button_url = EXCLUDED.menu_button_url,
    bot_inline_geo = EXCLUDED.bot_inline_geo,
    updated_at = now();

INSERT INTO public.peer_usernames (
    username_lower, username, peer_type, peer_id, active, editable, sort_order, updated_at
)
VALUES ('spambot', 'spambot', 'user', 1250000021, true, true, 0, now())
ON CONFLICT (username_lower) DO UPDATE SET
    username = EXCLUDED.username,
    peer_type = EXCLUDED.peer_type,
    peer_id = EXCLUDED.peer_id,
    active = EXCLUDED.active,
    editable = EXCLUDED.editable,
    updated_at = now();

INSERT INTO public.read_model_versions (model, owner_user_id, peer_type, peer_id, version, updated_at, hash)
VALUES
    ('contact_account', 1250000021, 'user', 1250000021, 1, now(), 2500002100001),
    ('channel_active_memberships', 1250000021, 'user', 1250000021, 1, now(), 2500002100002)
ON CONFLICT (model, owner_user_id, peer_type, peer_id) DO UPDATE SET
    version = GREATEST(public.read_model_versions.version, EXCLUDED.version),
    updated_at = now(),
    hash = EXCLUDED.hash;

-- notified_tier tracks the last tier @spambot proactively messaged the user
-- about; the notification sweep (ClaimDueSpamRestrictionNotifications) looks
-- for tier <> notified_tier. Defaulting to 0 means a brand-new restriction
-- row (tier escalated from the implicit 0) is correctly seen as "pending
-- notification" the first time it is written.
ALTER TABLE public.spam_restrictions ADD COLUMN notified_tier smallint NOT NULL DEFAULT 0;

CREATE INDEX spam_restrictions_pending_notify_idx ON public.spam_restrictions (updated_at)
  WHERE tier <> notified_tier;
