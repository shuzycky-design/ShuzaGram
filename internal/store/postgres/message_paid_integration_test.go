package postgres

import (
	"context"
	"testing"

	"telesrv/internal/domain"
)

// TestPaidPrivateMessageSettlesAtomicallyPostgres covers the fix for the "paid
// private messages have no logic at all" bug: sending a message to a peer who
// requires paid Stars must actually debit the sender, credit the recipient,
// and only ever do both together with the message itself.
func TestPaidPrivateMessageSettlesAtomicallyPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	users := NewUserStore(pool)
	suffix := randomSuffix(t)
	sender, err := users.Create(ctx, domain.User{AccessHash: 91101, Phone: "+1665951" + suffix + "01", FirstName: "PaidSender"})
	if err != nil {
		t.Fatalf("create sender: %v", err)
	}
	recipient, err := users.Create(ctx, domain.User{AccessHash: 91102, Phone: "+1665951" + suffix + "02", FirstName: "PaidRecipient"})
	if err != nil {
		t.Fatalf("create recipient: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM stars_transactions WHERE user_id=ANY($1::bigint[])", []int64{sender.ID, recipient.ID})
		_, _ = pool.Exec(ctx, "DELETE FROM stars_balances WHERE user_id=ANY($1::bigint[])", []int64{sender.ID, recipient.ID})
		_, _ = pool.Exec(ctx, "DELETE FROM message_boxes WHERE owner_user_id=ANY($1::bigint[])", []int64{sender.ID, recipient.ID})
		_, _ = pool.Exec(ctx, "DELETE FROM private_messages WHERE sender_user_id=$1", sender.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id=ANY($1::bigint[])", []int64{sender.ID, recipient.ID})
	})

	stars := NewStarsStore(pool)
	if _, err := stars.Credit(ctx, sender.ID, 100, domain.StarsReasonAdjust, domain.Peer{}, 1, "seed", ""); err != nil {
		t.Fatalf("seed sender balance: %v", err)
	}

	messages := NewMessageStore(pool)

	// Insufficient balance: the send must fail, and fail with nothing charged
	// and no message created -- the debit and the send are one transaction.
	_, err = messages.SendPrivateText(ctx, domain.SendPrivateTextRequest{
		SenderUserID: sender.ID, RecipientUserID: recipient.ID, RandomID: 910001,
		Message: "too expensive", Date: 2, PaidStars: 1000,
	})
	if err == nil {
		t.Fatalf("send with insufficient balance: want error, got success")
	}
	var senderBalance int64
	if err := pool.QueryRow(ctx, "SELECT balance FROM stars_balances WHERE user_id=$1", sender.ID).Scan(&senderBalance); err != nil {
		t.Fatalf("read sender balance after failed send: %v", err)
	}
	if senderBalance != 100 {
		t.Fatalf("sender balance after failed paid send = %d, want unchanged 100", senderBalance)
	}
	var messageCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM private_messages WHERE sender_user_id=$1 AND random_id=$2", sender.ID, int64(910001)).Scan(&messageCount); err != nil {
		t.Fatalf("count messages after failed send: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("messages after failed paid send = %d, want 0 (rolled back with the debit)", messageCount)
	}

	// Sufficient balance: the send succeeds, the sender is debited exactly
	// the paid amount, and the recipient is credited exactly that amount.
	res, err := messages.SendPrivateText(ctx, domain.SendPrivateTextRequest{
		SenderUserID: sender.ID, RecipientUserID: recipient.ID, RandomID: 910002,
		Message: "worth it", Date: 3, PaidStars: 30,
	})
	if err != nil {
		t.Fatalf("send paid message: %v", err)
	}
	if res.Duplicate {
		t.Fatalf("first paid send reported Duplicate = true")
	}
	if err := pool.QueryRow(ctx, "SELECT balance FROM stars_balances WHERE user_id=$1", sender.ID).Scan(&senderBalance); err != nil {
		t.Fatalf("read sender balance after paid send: %v", err)
	}
	if senderBalance != 70 {
		t.Fatalf("sender balance after paid send = %d, want 70 (100 - 30)", senderBalance)
	}
	var recipientBalance int64
	if err := pool.QueryRow(ctx, "SELECT balance FROM stars_balances WHERE user_id=$1", recipient.ID).Scan(&recipientBalance); err != nil {
		t.Fatalf("read recipient balance after paid send: %v", err)
	}
	if recipientBalance != 30 {
		t.Fatalf("recipient balance after paid send = %d, want 30", recipientBalance)
	}
	var senderTxnAmount, recipientTxnAmount int64
	var senderTxnReason, recipientTxnReason string
	if err := pool.QueryRow(ctx, "SELECT amount, reason FROM stars_transactions WHERE user_id=$1 ORDER BY id DESC LIMIT 1", sender.ID).
		Scan(&senderTxnAmount, &senderTxnReason); err != nil {
		t.Fatalf("read sender ledger entry: %v", err)
	}
	if senderTxnAmount != -30 || senderTxnReason != string(domain.StarsReasonPaidMessage) {
		t.Fatalf("sender ledger entry = %d/%s, want -30/%s", senderTxnAmount, senderTxnReason, domain.StarsReasonPaidMessage)
	}
	if err := pool.QueryRow(ctx, "SELECT amount, reason FROM stars_transactions WHERE user_id=$1 ORDER BY id DESC LIMIT 1", recipient.ID).
		Scan(&recipientTxnAmount, &recipientTxnReason); err != nil {
		t.Fatalf("read recipient ledger entry: %v", err)
	}
	if recipientTxnAmount != 30 || recipientTxnReason != string(domain.StarsReasonPaidMessage) {
		t.Fatalf("recipient ledger entry = %d/%s, want 30/%s", recipientTxnAmount, recipientTxnReason, domain.StarsReasonPaidMessage)
	}

	// An exact random_id replay must not charge a second time.
	replay, err := messages.SendPrivateText(ctx, domain.SendPrivateTextRequest{
		SenderUserID: sender.ID, RecipientUserID: recipient.ID, RandomID: 910002,
		Message: "worth it", Date: 3, PaidStars: 30,
	})
	if err != nil {
		t.Fatalf("replay paid send: %v", err)
	}
	if !replay.Duplicate {
		t.Fatalf("replay paid send Duplicate = false, want true")
	}
	if err := pool.QueryRow(ctx, "SELECT balance FROM stars_balances WHERE user_id=$1", sender.ID).Scan(&senderBalance); err != nil {
		t.Fatalf("read sender balance after replay: %v", err)
	}
	if senderBalance != 70 {
		t.Fatalf("sender balance after replay = %d, want unchanged 70", senderBalance)
	}
}
