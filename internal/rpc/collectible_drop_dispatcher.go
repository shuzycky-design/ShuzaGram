package rpc

import (
	"context"
	"time"

	"go.uber.org/zap"
)

const defaultCollectibleDropTick = 30 * time.Second

// CollectibleDropDispatcher activates deferred star gift collectible drops:
// an admin can publish a collectible pool (models/patterns/backdrops +
// supply/upgrade price) as a draft ahead of time, scheduled for a future
// instant, instead of it only ever being able to go live the moment they
// click publish. This dispatcher is the thing that actually flips such a
// draft live once its scheduled_publish_at arrives -- see
// internal/store/postgres/star_gift_collectibles.go's PublishCollectibleRevision
// for how a draft is written in the first place. The plain (non-collectible)
// gift itself is unaffected either way: nothing here ever touches
// star_gift_catalog except the one row whose collectible pool just opened.
type CollectibleDropDispatcher struct {
	router   *Router
	log      *zap.Logger
	interval time.Duration
}

func NewCollectibleDropDispatcher(router *Router, log *zap.Logger, interval time.Duration) *CollectibleDropDispatcher {
	if log == nil {
		log = zap.NewNop()
	}
	if interval <= 0 {
		interval = defaultCollectibleDropTick
	}
	return &CollectibleDropDispatcher{router: router, log: log, interval: interval}
}

func (d *CollectibleDropDispatcher) Run(ctx context.Context) {
	if d == nil || d.router == nil {
		return
	}
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.DispatchOnce(ctx)
		}
	}
}

func (d *CollectibleDropDispatcher) DispatchOnce(ctx context.Context) {
	if d == nil || d.router == nil || d.router.deps.Gifts == nil {
		return
	}
	giftIDs, err := d.router.deps.Gifts.ActivateDueCollectibleRevisions(ctx)
	if err != nil {
		d.log.Error("activate due collectible revisions", zap.Error(err))
		return
	}
	for _, giftID := range giftIDs {
		d.log.Info("collectible drop activated", zap.Int64("gift_id", giftID))
	}
}
