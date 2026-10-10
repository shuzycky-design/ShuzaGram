DROP INDEX IF EXISTS public.spam_restrictions_pending_notify_idx;
ALTER TABLE public.spam_restrictions DROP COLUMN IF EXISTS notified_tier;

DELETE FROM public.read_model_versions WHERE peer_type = 'user' AND peer_id = 1250000021;
DELETE FROM public.peer_usernames WHERE username_lower = 'spambot';
DELETE FROM public.bots WHERE bot_user_id = 1250000021;
DELETE FROM public.users WHERE id = 1250000021;
