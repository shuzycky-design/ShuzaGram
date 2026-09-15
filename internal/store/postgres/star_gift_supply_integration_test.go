package postgres

import (
	"context"
	"testing"
	"time"

	"telesrv/internal/domain"
)

// TestSetCatalogSupplyPostgres exercises the in-place "X of Y sold" edit
// against a real database (the active revision + catalog row FK/columns
// this touches aren't obvious from the Go code alone -- see the store
// method's doc comment for why it deliberately doesn't mint a new revision).
func TestSetCatalogSupplyPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	suffix := randomSuffix(t)
	gifts := NewStarGiftStore(pool)
	baseDocumentID := time.Now().UnixNano() & 0x7ffffffffffff000
	entry, err := gifts.CreateCatalogRevision(ctx, domain.StarGiftCatalogWrite{
		Title: "Supply Test", Stars: 50, ConvertStars: 25, Enabled: true,
		Document:  collectibleTestDocument(baseDocumentID, "gift.tgs"),
		Blob:      collectibleTestBlob(baseDocumentID, "gift"),
		Animation: collectibleTestAnimation("gift.tgs"),
		Actor:     "integration", CommandID: "catalog-supply-" + suffix,
	})
	if err != nil {
		t.Fatalf("create catalog gift: %v", err)
	}
	giftID := entry.Gift.ID

	changed, err := gifts.SetCatalogSupply(ctx, giftID, true, 1000, 200)
	if err != nil || !changed {
		t.Fatalf("set supply: changed=%v err=%v", changed, err)
	}
	gift, found, err := gifts.CatalogGift(ctx, giftID)
	if err != nil || !found || !gift.Limited || gift.AvailabilityTotal != 1000 || gift.AvailabilityRemains != 800 || gift.SoldOut {
		t.Fatalf("gift after set supply = found:%v err:%v value:%+v", found, err, gift)
	}

	// A no-op call (same values) must report unchanged, not error.
	changed, err = gifts.SetCatalogSupply(ctx, giftID, true, 1000, 200)
	if err != nil || changed {
		t.Fatalf("no-op set supply: changed=%v err=%v, want changed=false", changed, err)
	}

	// Selling out (issued == total) must set sold_out.
	if _, err := gifts.SetCatalogSupply(ctx, giftID, true, 1000, 1000); err != nil {
		t.Fatalf("set supply to sold out: %v", err)
	}
	gift, _, err = gifts.CatalogGift(ctx, giftID)
	if err != nil || !gift.SoldOut || gift.AvailabilityRemains != 0 {
		t.Fatalf("gift after sold out = err:%v value:%+v", err, gift)
	}

	// Clearing the cap entirely (Limited=false) drops the restriction and zeroes
	// the tracked total/remains -- a plain, unrestricted gift again.
	if _, err := gifts.SetCatalogSupply(ctx, giftID, false, 0, 0); err != nil {
		t.Fatalf("clear supply cap: %v", err)
	}
	gift, _, err = gifts.CatalogGift(ctx, giftID)
	if err != nil || gift.Limited || gift.SoldOut || gift.AvailabilityTotal != 0 || gift.AvailabilityRemains != 0 {
		t.Fatalf("gift after clearing cap = err:%v value:%+v", err, gift)
	}

	// Invalid inputs are rejected before anything is touched.
	for _, invalid := range []struct {
		limited       bool
		total, issued int
	}{
		{true, 0, 0},   // limited requires total > 0
		{true, 10, 11}, // issued cannot exceed total
		{true, 10, -1}, // issued cannot be negative
	} {
		if _, err := gifts.SetCatalogSupply(ctx, giftID, invalid.limited, invalid.total, invalid.issued); err == nil {
			t.Fatalf("invalid supply %+v was accepted", invalid)
		}
	}

	if _, err := gifts.SetCatalogSupply(ctx, giftID+999999, true, 10, 0); err == nil {
		t.Fatal("set supply on a nonexistent gift was accepted")
	}
}
