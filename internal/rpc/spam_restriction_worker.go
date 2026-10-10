package rpc

import (
	"context"
	"time"

	"go.uber.org/zap"

	"telesrv/internal/domain"
)

type spamRestrictionDecayService interface {
	SweepDueSpamRestrictionDecay(ctx context.Context, now time.Time, limit int) (int, error)
}

type spamRestrictionNotificationService interface {
	ClaimDueSpamRestrictionNotifications(ctx context.Context, limit int) ([]domain.SpamRestrictionNotification, error)
}

// RunSpamRestrictionDecay periodically auto-lifts (clears to tier 0) any
// non-manual-override spam restriction whose last report is older than the
// admin-configured decay window. Enforcement is a pure live read
// (ensureSpamRestrictionAllowed), never pushed to clients, so no
// cache/projection invalidation is needed here.
func (r *Router) RunSpamRestrictionDecay(ctx context.Context, interval time.Duration, batch int) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if batch <= 0 {
		batch = 500
	}
	r.runSpamRestrictionDecayOnce(ctx, batch)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runSpamRestrictionDecayOnce(ctx, batch)
		}
	}
}

func (r *Router) runSpamRestrictionDecayOnce(ctx context.Context, batch int) {
	svc, ok := r.deps.SpamRestrictions.(spamRestrictionDecayService)
	if !ok {
		return
	}
	now := r.clock.Now().UTC()
	sweepCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	cleared, err := svc.SweepDueSpamRestrictionDecay(sweepCtx, now, batch)
	cancel()
	if err != nil {
		r.log.Warn("spam restriction decay sweep failed", zap.Int("cleared", cleared), zap.Error(err))
	}
}

// RunSpamRestrictionNotifications periodically has @spambot message any user
// whose restriction tier changed (escalation, admin override, or decay)
// since it last told them. Independent of the decay sweep's own interval so
// a user hears about an escalation promptly even with a long decay window.
func (r *Router) RunSpamRestrictionNotifications(ctx context.Context, interval time.Duration, batch int) {
	if interval <= 0 {
		interval = time.Minute
	}
	if batch <= 0 {
		batch = 200
	}
	r.runSpamRestrictionNotificationsOnce(ctx, batch)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runSpamRestrictionNotificationsOnce(ctx, batch)
		}
	}
}

func (r *Router) runSpamRestrictionNotificationsOnce(ctx context.Context, batch int) {
	svc, ok := r.deps.SpamRestrictions.(spamRestrictionNotificationService)
	if !ok || r.deps.SpamBotNotifier == nil {
		return
	}
	claimCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	pending, err := svc.ClaimDueSpamRestrictionNotifications(claimCtx, batch)
	cancel()
	if err != nil {
		r.log.Warn("claim spam restriction notifications failed", zap.Error(err))
		return
	}
	for _, n := range pending {
		notifyCtx, notifyCancel := context.WithTimeout(ctx, 15*time.Second)
		err := r.deps.SpamBotNotifier.NotifySpamRestrictionTierChanged(notifyCtx, n.UserID, n.Tier)
		notifyCancel()
		if err != nil {
			r.log.Warn("spam restriction tier change notification failed",
				zap.Int64("user_id", n.UserID), zap.Int("tier", int(n.Tier)), zap.Error(err))
		}
	}
}
