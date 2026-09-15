package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"telesrv/internal/domain"
)

// TestDeleteStarGiftPostgres exercises the full purge -- plain gift, its
// collectible pool, and two owners (one who only bought the plain gift, one
// who also upgraded it) -- against a real database, since the FK-ordering
// this depends on (see star_gift_delete.go's doc comments) can't be verified
// by reading the code alone.
func TestDeleteStarGiftPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	suffix := randomSuffix(t)
	users := NewUserStore(pool)
	sender := createTestUser(t, ctx, users, "+1778"+suffix+"51", "DeleteSender", "")
	buyerOnly := createTestUser(t, ctx, users, "+1778"+suffix+"52", "DeleteBuyerOnly", "")
	upgrader := createTestUser(t, ctx, users, "+1778"+suffix+"53", "DeleteUpgrader", "")
	buyerPeer := domain.Peer{Type: domain.PeerTypeUser, ID: buyerOnly.ID}
	upgraderPeer := domain.Peer{Type: domain.PeerTypeUser, ID: upgrader.ID}

	gifts := NewStarGiftStore(pool)
	baseDocumentID := time.Now().UnixNano() & 0x7ffffffffffff000
	entry, err := gifts.CreateCatalogRevision(ctx, domain.StarGiftCatalogWrite{
		Title: "Doomed Gift", Stars: 50, ConvertStars: 25, Enabled: true,
		Document:  collectibleTestDocument(baseDocumentID, "gift.tgs"),
		Blob:      collectibleTestBlob(baseDocumentID, "gift"),
		Animation: collectibleTestAnimation("gift.tgs"),
		Actor:     "integration", CommandID: "catalog-delete-" + suffix,
	})
	if err != nil {
		t.Fatalf("create catalog gift: %v", err)
	}
	giftID := entry.Gift.ID

	poolRevision, err := gifts.PublishCollectibleRevision(ctx, scheduledCollectibleWrite(giftID, suffix, 0))
	if err != nil {
		t.Fatalf("publish collectible pool: %v", err)
	}

	messages := NewMessageStore(pool)
	buyerSaved := createCollectibleSavedGift(t, ctx, messages, gifts, entry.Gift, domain.SavedStarGift{
		Owner: buyerPeer, FromUserID: sender.ID, GiftID: giftID, RevisionID: entry.Gift.RevisionID,
		Date: 1700002000, ConvertStars: 25, Message: "plain only",
	})
	upgraderSaved := createCollectibleSavedGift(t, ctx, messages, gifts, entry.Gift, domain.SavedStarGift{
		Owner: upgraderPeer, FromUserID: sender.ID, GiftID: giftID, RevisionID: entry.Gift.RevisionID,
		Date: 1700002001, ConvertStars: 25, Message: "will upgrade",
	})

	stars := NewStarsStore(pool)
	if _, _, err := stars.EnsureGrant(ctx, upgrader.ID, 1000, 1700002002); err != nil {
		t.Fatalf("grant upgrade stars: %v", err)
	}
	upgrades := NewStarGiftUpgradeStore(pool, messages)
	upgraded, err := upgrades.UpgradeStarGift(ctx, domain.StarGiftUpgradeRequest{
		UserID: upgrader.ID, Ref: domain.SavedStarGiftRef{Owner: upgraderPeer, MsgID: upgraderSaved.MsgID},
		KeepOriginalDetails: true, ChargeStars: poolRevision.UpgradeStars, FormID: 992,
		CommandKey: "paid-delete-" + suffix, Date: 1700002003,
	})
	if err != nil {
		t.Fatalf("upgrade star gift: %v", err)
	}

	// --- Preview: both owners present, only the upgrader has an upgrade refund
	// (neither went through the real purchase RPC here, so PurchaseStars is 0
	// for both -- createCollectibleSavedGift is a fixture helper, not the
	// purchase flow itself; that path is already covered by
	// TestStarGiftCollectibleUpgradeAggregatePostgres above).
	preview, err := gifts.PreviewStarGiftDelete(ctx, giftID)
	if err != nil {
		t.Fatalf("preview delete: %v", err)
	}
	if preview.Title != "Doomed Gift" || preview.Owners != 2 || preview.UniqueOwners != 1 {
		t.Fatalf("preview = %+v", preview)
	}
	if len(preview.Blockers) != 0 {
		t.Fatalf("preview blockers = %+v, want none", preview.Blockers)
	}
	var upgraderRefund, buyerRefund *domain.StarGiftOwnerRefund
	for i := range preview.Refunds {
		switch preview.Refunds[i].UserID {
		case upgrader.ID:
			upgraderRefund = &preview.Refunds[i]
		case buyerOnly.ID:
			buyerRefund = &preview.Refunds[i]
		}
	}
	if upgraderRefund == nil || upgraderRefund.UpgradeStars != poolRevision.UpgradeStars || upgraderRefund.PurchaseStars != 0 {
		t.Fatalf("upgrader refund = %+v, want upgrade_stars=%d", upgraderRefund, poolRevision.UpgradeStars)
	}
	if buyerRefund != nil {
		t.Fatalf("buyer-only owner should have no refund (no purchase_commands row via this fixture): %+v", buyerRefund)
	}

	// --- Delete: everything for this gift must be gone afterward, with no FK errors.
	result, err := gifts.DeleteStarGift(ctx, giftID)
	if err != nil {
		t.Fatalf("delete star gift: %v", err)
	}
	if result.Owners != 2 || result.UniqueOwners != 1 || result.Title != "Doomed Gift" {
		t.Fatalf("delete result = %+v", result)
	}

	var remaining int
	checks := []struct {
		name string
		sql  string
	}{
		{"catalog", `SELECT count(*) FROM star_gift_catalog WHERE gift_id=$1`},
		{"catalog_revisions", `SELECT count(*) FROM star_gift_catalog_revisions WHERE gift_id=$1`},
		{"collectible_revisions", `SELECT count(*) FROM star_gift_collectible_revisions WHERE gift_id=$1`},
		{"unique_star_gifts", `SELECT count(*) FROM unique_star_gifts WHERE gift_id=$1`},
		{"peer_star_gifts", `SELECT count(*) FROM peer_star_gifts WHERE gift_id=$1`},
	}
	for _, check := range checks {
		if err := pool.QueryRow(ctx, check.sql, giftID).Scan(&remaining); err != nil || remaining != 0 {
			t.Fatalf("%s remaining=%d err=%v, want 0", check.name, remaining, err)
		}
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM star_gift_collectible_models WHERE collectible_revision_id=$1`,
		poolRevision.ID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("collectible_models remaining=%d err=%v, want 0 (should cascade)", remaining, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM star_gift_upgrade_commands WHERE unique_gift_id=$1`,
		upgraded.Unique.ID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("upgrade_commands remaining=%d err=%v, want 0", remaining, err)
	}

	// Message history rows themselves are untouched (only the saved-gift link
	// cascades) -- deleting a gift from the catalog must not silently corrupt
	// old chat history.
	for _, saved := range []struct {
		owner domain.Peer
		msgID int
	}{{buyerPeer, buyerSaved.MsgID}, {upgraderPeer, upgraderSaved.MsgID}} {
		list, err := messages.GetByIDs(ctx, saved.owner.ID, []int{saved.msgID})
		if err != nil || len(list.Messages) != 1 {
			t.Fatalf("source message for %+v msg %d should survive: messages=%d err=%v", saved.owner, saved.msgID, len(list.Messages), err)
		}
	}

	if _, err := gifts.PreviewStarGiftDelete(ctx, giftID); !errors.Is(err, domain.ErrStarGiftNotFound) {
		t.Fatalf("preview after delete err=%v, want ErrStarGiftNotFound", err)
	}
	if _, err := gifts.DeleteStarGift(ctx, giftID); !errors.Is(err, domain.ErrStarGiftNotFound) {
		t.Fatalf("re-delete err=%v, want ErrStarGiftNotFound", err)
	}
}

// TestDeleteStarGiftBlockedByActiveListingPostgres confirms the deliberate
// refusal to cascade through secondary-market state -- see
// domain.StarGiftDeleteResult.Blockers.
func TestDeleteStarGiftBlockedByActiveListingPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	suffix := randomSuffix(t)
	users := NewUserStore(pool)
	sender := createTestUser(t, ctx, users, "+1778"+suffix+"61", "BlockedSender", "")
	owner := createTestUser(t, ctx, users, "+1778"+suffix+"62", "BlockedOwner", "")
	ownerPeer := domain.Peer{Type: domain.PeerTypeUser, ID: owner.ID}

	gifts := NewStarGiftStore(pool)
	baseDocumentID := time.Now().UnixNano() & 0x7ffffffffffff000
	entry, err := gifts.CreateCatalogRevision(ctx, domain.StarGiftCatalogWrite{
		Title: "Listed Gift", Stars: 50, ConvertStars: 25, Enabled: true,
		Document:  collectibleTestDocument(baseDocumentID, "gift.tgs"),
		Blob:      collectibleTestBlob(baseDocumentID, "gift"),
		Animation: collectibleTestAnimation("gift.tgs"),
		Actor:     "integration", CommandID: "catalog-blocked-" + suffix,
	})
	if err != nil {
		t.Fatalf("create catalog gift: %v", err)
	}
	giftID := entry.Gift.ID
	poolRevision, err := gifts.PublishCollectibleRevision(ctx, scheduledCollectibleWrite(giftID, suffix, 0))
	if err != nil {
		t.Fatalf("publish collectible pool: %v", err)
	}
	messages := NewMessageStore(pool)
	saved := createCollectibleSavedGift(t, ctx, messages, gifts, entry.Gift, domain.SavedStarGift{
		Owner: ownerPeer, FromUserID: sender.ID, GiftID: giftID, RevisionID: entry.Gift.RevisionID,
		Date: 1700003000, ConvertStars: 25, Message: "listed",
	})
	stars := NewStarsStore(pool)
	if _, _, err := stars.EnsureGrant(ctx, owner.ID, 1000, 1700003001); err != nil {
		t.Fatalf("grant upgrade stars: %v", err)
	}
	upgrades := NewStarGiftUpgradeStore(pool, messages)
	upgraded, err := upgrades.UpgradeStarGift(ctx, domain.StarGiftUpgradeRequest{
		UserID: owner.ID, Ref: domain.SavedStarGiftRef{Owner: ownerPeer, MsgID: saved.MsgID},
		KeepOriginalDetails: true, ChargeStars: poolRevision.UpgradeStars, FormID: 993,
		CommandKey: "paid-blocked-" + suffix, Date: 1700003002,
	})
	if err != nil {
		t.Fatalf("upgrade star gift: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO star_gift_listings (unique_gift_id, seller_peer_type, seller_peer_id, currency, amount, listed_at, updated_at)
VALUES ($1, 'user', $2, 'XTR', 500, extract(epoch from now())::int, extract(epoch from now())::int)`,
		upgraded.Unique.ID, owner.ID); err != nil {
		t.Fatalf("seed active listing: %v", err)
	}

	preview, err := gifts.PreviewStarGiftDelete(ctx, giftID)
	if err != nil {
		t.Fatalf("preview delete: %v", err)
	}
	if preview.Blockers["listings"] != 1 {
		t.Fatalf("preview blockers = %+v, want listings=1", preview.Blockers)
	}

	if _, err := gifts.DeleteStarGift(ctx, giftID); !errors.Is(err, domain.ErrStarGiftDeleteBlocked) {
		t.Fatalf("delete err=%v, want ErrStarGiftDeleteBlocked", err)
	}
	var stillThere int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM star_gift_catalog WHERE gift_id=$1`, giftID).Scan(&stillThere); err != nil || stillThere != 1 {
		t.Fatalf("blocked delete must not have touched anything: remaining=%d err=%v", stillThere, err)
	}
}
