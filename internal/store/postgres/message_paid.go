package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"telesrv/internal/domain"
)

// debitPrivatePaidMessage atomically settles one paid private message: the
// sender's balance is locked and checked again here (the RPC boundary's own
// check, in ensurePrivateContactAllowed, is necessarily a separate earlier
// read and cannot be trusted as the actual authorization), then debited, and
// the full amount is credited straight to the recipient -- unlike a channel's
// Direct Messages price (see paidMessageChannelCommissionPermille in
// channel_monoforum.go), a private message has no platform-owned channel
// revenue ledger to split proceeds into, so the whole payment is the
// recipient's.
//
// Caller must already hold the per-user-pair lock (lockUsersForUpdate) that
// sendPrivateTextOnce takes before this runs, so this cannot deadlock against
// a concurrent send between the same two users.
func debitPrivatePaidMessage(ctx context.Context, tx pgx.Tx, senderUserID, recipientUserID, stars int64, date int) error {
	if senderUserID == 0 || recipientUserID == 0 || stars <= 0 {
		return domain.ErrStarsInvalidAmount
	}
	var senderBalance int64
	if err := tx.QueryRow(ctx, `SELECT balance FROM stars_balances WHERE user_id = $1 FOR UPDATE`, senderUserID).
		Scan(&senderBalance); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrStarsInsufficient
		}
		return fmt.Errorf("lock paid-message sender balance: %w", err)
	}
	if senderBalance < stars {
		return domain.ErrStarsInsufficient
	}
	if _, err := tx.Exec(ctx, `UPDATE stars_balances SET balance = balance - $2, updated_at = now() WHERE user_id = $1`,
		senderUserID, stars); err != nil {
		return fmt.Errorf("debit paid-message sender balance: %w", err)
	}
	if err := insertStarsTxn(ctx, tx, senderUserID, -stars, domain.StarsReasonPaidMessage,
		domain.Peer{Type: domain.PeerTypeUser, ID: recipientUserID}, date, "Paid message", ""); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO stars_balances (user_id, balance, updated_at) VALUES ($1, $2, now())
ON CONFLICT (user_id) DO UPDATE SET balance = stars_balances.balance + EXCLUDED.balance, updated_at = now()`,
		recipientUserID, stars); err != nil {
		return fmt.Errorf("credit paid-message recipient balance: %w", err)
	}
	if err := insertStarsTxn(ctx, tx, recipientUserID, stars, domain.StarsReasonPaidMessage,
		domain.Peer{Type: domain.PeerTypeUser, ID: senderUserID}, date, "Paid message", ""); err != nil {
		return err
	}
	return nil
}
