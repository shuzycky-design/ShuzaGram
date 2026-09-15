package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"telesrv/internal/domain"
)

// blockingCounts runs the same "is there any secondary-market/administrative
// state this deletion refuses to touch" check for both PreviewStarGiftDelete
// (report only) and DeleteStarGift (refuse to run). See
// domain.StarGiftDeleteResult.Blockers for why these are never cascaded
// through automatically.
func blockingCounts(ctx context.Context, db interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, giftID int64) (map[string]int, error) {
	var (
		listings, offers, sales, transfers, withdrawals       int
		craftResults, adminGrants, dropDetails, prepaid       int
		conversions, auctionAcquired, purchaseForms, auctions int
		emojiStatus                                           int
	)
	err := db.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM star_gift_listings WHERE unique_gift_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_offers WHERE unique_gift_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_sales WHERE unique_gift_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_transfer_commands WHERE unique_gift_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_withdrawal_requests WHERE unique_gift_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_craft_commands WHERE gift_id=$1 OR result_unique_gift_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_admin_grant_commands WHERE saved_gift_id IN (SELECT id FROM peer_star_gifts WHERE gift_id=$1) OR unique_gift_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_drop_details_commands WHERE saved_gift_id IN (SELECT id FROM peer_star_gifts WHERE gift_id=$1) OR unique_gift_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_prepaid_upgrade_commands WHERE saved_gift_id IN (SELECT id FROM peer_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_conversions WHERE saved_gift_id IN (SELECT id FROM peer_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_auction_acquired WHERE saved_gift_id IN (SELECT id FROM peer_star_gifts WHERE gift_id=$1)),
  (SELECT count(*) FROM star_gift_purchase_forms WHERE gift_id=$1),
  (SELECT count(*) FROM star_gift_auctions WHERE gift_id=$1),
  (SELECT count(*) FROM users WHERE emoji_status_collectible_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1))
`, giftID).Scan(&listings, &offers, &sales, &transfers, &withdrawals, &craftResults, &adminGrants,
		&dropDetails, &prepaid, &conversions, &auctionAcquired, &purchaseForms, &auctions, &emojiStatus)
	if err != nil {
		return nil, fmt.Errorf("check star gift delete blockers: %w", err)
	}
	blockers := map[string]int{}
	add := func(name string, n int) {
		if n > 0 {
			blockers[name] = n
		}
	}
	add("listings", listings)
	add("offers", offers)
	add("sales", sales)
	add("transfers", transfers)
	add("withdrawal_requests", withdrawals)
	add("craft_commands", craftResults)
	add("admin_grant_commands", adminGrants)
	add("drop_details_commands", dropDetails)
	add("prepaid_upgrade_commands", prepaid)
	add("conversions", conversions)
	add("auction_acquired", auctionAcquired)
	add("purchase_forms", purchaseForms)
	add("auctions", auctions)
	add("emoji_status_users", emojiStatus)
	return blockers, nil
}

// PreviewStarGiftDelete computes what DeleteStarGift would do without deleting or
// crediting anything -- see the store.StarGiftStore interface doc comment.
func (s *StarGiftStore) PreviewStarGiftDelete(ctx context.Context, giftID int64) (domain.StarGiftDeleteResult, error) {
	if giftID <= 0 {
		return domain.StarGiftDeleteResult{}, domain.ErrStarGiftInvalid
	}
	result := domain.StarGiftDeleteResult{GiftID: giftID}
	if err := s.db.QueryRow(ctx, `
SELECT r.title FROM star_gift_catalog c JOIN star_gift_catalog_revisions r ON r.id=c.active_revision_id
WHERE c.gift_id=$1`, giftID).Scan(&result.Title); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.StarGiftDeleteResult{}, domain.ErrStarGiftNotFound
		}
		return domain.StarGiftDeleteResult{}, fmt.Errorf("get star gift for delete preview: %w", err)
	}

	blockers, err := blockingCounts(ctx, s.db, giftID)
	if err != nil {
		return domain.StarGiftDeleteResult{}, err
	}
	result.Blockers = blockers

	rows, err := s.db.Query(ctx, `
SELECT p.owner_peer_type, p.owner_peer_id, p.unique_gift_id IS NOT NULL,
       COALESCE((SELECT SUM(charge_stars) FROM star_gift_purchase_commands WHERE saved_gift_id=p.id), 0),
       COALESCE((SELECT SUM(charge_stars) FROM star_gift_upgrade_commands WHERE source_saved_gift_id=p.id), 0)
FROM peer_star_gifts p WHERE p.gift_id=$1`, giftID)
	if err != nil {
		return domain.StarGiftDeleteResult{}, fmt.Errorf("list star gift owners for delete preview: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ownerType string
		var ownerID int64
		var uniqued bool
		var purchaseStars, upgradeStars int64
		if err := rows.Scan(&ownerType, &ownerID, &uniqued, &purchaseStars, &upgradeStars); err != nil {
			return domain.StarGiftDeleteResult{}, fmt.Errorf("scan star gift owner for delete preview: %w", err)
		}
		result.Owners++
		if uniqued {
			result.UniqueOwners++
		}
		if ownerType == "user" && (purchaseStars > 0 || upgradeStars > 0) {
			result.Refunds = append(result.Refunds, domain.StarGiftOwnerRefund{
				UserID: ownerID, PurchaseStars: purchaseStars, UpgradeStars: upgradeStars,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return domain.StarGiftDeleteResult{}, fmt.Errorf("iterate star gift owners for delete preview: %w", err)
	}
	return result, nil
}

// DeleteStarGift permanently removes a gift and every trace of it -- see the
// store.StarGiftStore interface doc comment. It refuses to run (returning
// domain.ErrStarGiftDeleteBlocked) if PreviewStarGiftDelete-equivalent
// blockers exist; callers that need the refund breakdown to credit Stars
// first should call PreviewStarGiftDelete before this, not after.
func (s *StarGiftStore) DeleteStarGift(ctx context.Context, giftID int64) (domain.StarGiftDeleteResult, error) {
	if giftID <= 0 {
		return domain.StarGiftDeleteResult{}, domain.ErrStarGiftInvalid
	}
	result := domain.StarGiftDeleteResult{GiftID: giftID}
	err := withTx(ctx, s.db, "delete star gift", func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
SELECT r.title FROM star_gift_catalog c JOIN star_gift_catalog_revisions r ON r.id=c.active_revision_id
WHERE c.gift_id=$1 FOR UPDATE`, giftID).Scan(&result.Title); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrStarGiftNotFound
			}
			return fmt.Errorf("lock star gift for delete: %w", err)
		}

		blockers, err := blockingCounts(ctx, tx, giftID)
		if err != nil {
			return err
		}
		if len(blockers) > 0 {
			result.Blockers = blockers
			return domain.ErrStarGiftDeleteBlocked
		}

		if err := tx.QueryRow(ctx, `
SELECT count(*), count(*) FILTER (WHERE unique_gift_id IS NOT NULL) FROM peer_star_gifts WHERE gift_id=$1`, giftID).
			Scan(&result.Owners, &result.UniqueOwners); err != nil {
			return fmt.Errorf("count star gift owners for delete: %w", err)
		}

		// Audit/idempotency records tied to this gift_id / these owners' unique gifts.
		if _, err := tx.Exec(ctx, `DELETE FROM star_gift_purchase_commands WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("delete star gift purchase commands: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM star_gift_user_purchases WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("delete star gift user purchases: %w", err)
		}
		if _, err := tx.Exec(ctx, `
DELETE FROM star_gift_upgrade_commands WHERE unique_gift_id IN (SELECT id FROM unique_star_gifts WHERE gift_id=$1)`, giftID); err != nil {
			return fmt.Errorf("delete star gift upgrade commands: %w", err)
		}

		// Break the peer_star_gifts <-> unique_star_gifts circular RESTRICT dependency
		// from the peer side (unique_star_gifts.source_saved_gift_id is NOT NULL).
		if _, err := tx.Exec(ctx, `UPDATE peer_star_gifts SET unique_gift_id=NULL WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("detach star gift unique gifts: %w", err)
		}
		// star_gift_catalog.collectible_revision_id's FK is RESTRICT and not deferrable,
		// unlike active_revision_id below, so it must be cleared before the collectible
		// revisions can go. The BEFORE UPDATE trigger on this column is a no-op for NULL.
		if _, err := tx.Exec(ctx, `UPDATE star_gift_catalog SET collectible_revision_id=NULL WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("detach star gift active collectible revision: %w", err)
		}

		if _, err := tx.Exec(ctx, `DELETE FROM unique_star_gifts WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("delete unique star gifts: %w", err)
		}
		// The actual confiscation: owners lose the gift outright. Message refs,
		// collection memberships and channel notification jobs cascade.
		if _, err := tx.Exec(ctx, `DELETE FROM peer_star_gifts WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("delete peer star gifts: %w", err)
		}

		// From here on: both published collectible revisions are guarded immutable/
		// undeletable by telesrv_guard_collectible_revision, and -- this is the part
		// that isn't obvious from the schema -- star_gift_catalog.active_revision_id's
		// FK is declared DEFERRABLE INITIALLY DEFERRED but that clause is a no-op on
		// an ON DELETE RESTRICT constraint: per Postgres's own docs, "the essential
		// difference [between NO ACTION and RESTRICT] is that NO ACTION allows the
		// check to be deferred ..., whereas RESTRICT does not" -- so deleting the
		// referenced catalog_revisions row while active_revision_id still points to
		// it fails immediately, in this same transaction, no matter what order the
		// statements run in or when commit happens. session_replication_role=replica
		// disables the RESTRICT-enforcing trigger outright (not just its timing),
		// which is what actually makes this work -- SET LOCAL scopes it to this
		// transaction, reverting automatically at commit/rollback.
		// replica mode also disables the ON DELETE CASCADE triggers that would
		// otherwise clear star_gift_collectible_models/patterns/backdrops on their
		// own (CASCADE is trigger-implemented too), so those need an explicit
		// delete here instead of relying on the cascade.
		if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
			return fmt.Errorf("enter delete-privileged mode: %w", err)
		}
		// star_gift_collectible_preview_repairs references both gift_id (CASCADE)
		// and collectible_revision_id (RESTRICT) -- also silenced by replica mode,
		// same as the model/pattern/backdrop cascades above.
		if _, err := tx.Exec(ctx, `DELETE FROM star_gift_collectible_preview_repairs WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("delete star gift collectible preview repairs: %w", err)
		}
		for _, table := range []string{"star_gift_collectible_models", "star_gift_collectible_patterns", "star_gift_collectible_backdrops"} {
			if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE collectible_revision_id IN (SELECT id FROM star_gift_collectible_revisions WHERE gift_id=$1)`, giftID); err != nil {
				return fmt.Errorf("delete %s: %w", table, err)
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM star_gift_collectible_revisions WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("delete star gift collectible revisions: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM star_gift_catalog_revisions WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("delete star gift catalog revisions: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM star_gift_catalog WHERE gift_id=$1`, giftID); err != nil {
			return fmt.Errorf("delete star gift catalog entry: %w", err)
		}
		if _, err := tx.Exec(ctx, `SET LOCAL session_replication_role = DEFAULT`); err != nil {
			return fmt.Errorf("leave delete-privileged mode: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.StarGiftDeleteResult{}, err
	}
	return result, nil
}
