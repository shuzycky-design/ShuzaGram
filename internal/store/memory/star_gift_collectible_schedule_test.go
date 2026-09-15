package memory

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"telesrv/internal/domain"
)

func testCollectibleWrite(giftID int64, publishAt int64) domain.StarGiftCollectibleWrite {
	animation := &domain.StarGiftAnimation{JSON: []byte(`{}`), TGS: []byte{1}, SHA256: make([]byte, sha256.Size)}
	models := []domain.StarGiftCollectibleAttribute{
		{Kind: domain.StarGiftCollectibleModel, Name: "Regular", RarityKind: domain.StarGiftRarityPermille, RarityPermille: 500, Animation: animation,
			Document: &domain.Document{ID: 101, MimeType: "application/x-tgsticker", Attributes: []domain.DocumentAttribute{{Kind: domain.DocAttrSticker, Alt: "🎁"}}},
			Blob:     &domain.FileBlob{LocationKey: "model-1"}},
		{Kind: domain.StarGiftCollectibleModel, Name: "Regular Two", RarityKind: domain.StarGiftRarityPermille, RarityPermille: 500, Animation: animation,
			Document: &domain.Document{ID: 102, MimeType: "application/x-tgsticker", Attributes: []domain.DocumentAttribute{{Kind: domain.DocAttrSticker, Alt: "🎁"}}},
			Blob:     &domain.FileBlob{LocationKey: "model-2"}},
	}
	patterns := []domain.StarGiftCollectibleAttribute{
		{Kind: domain.StarGiftCollectiblePattern, Name: "Pattern", RarityKind: domain.StarGiftRarityPermille, RarityPermille: 500, Animation: animation,
			Document: &domain.Document{ID: 201, MimeType: "application/x-tgsticker", Attributes: []domain.DocumentAttribute{{Kind: domain.DocAttrCustomEmoji, Alt: "🎁", TextColor: true}}, Thumbs: []domain.PhotoSize{{Kind: domain.PhotoSizeKindPath, Type: "j", Bytes: []byte{1}}}},
			Blob:     &domain.FileBlob{LocationKey: "pattern-1"}},
		{Kind: domain.StarGiftCollectiblePattern, Name: "Pattern Two", RarityKind: domain.StarGiftRarityPermille, RarityPermille: 500, Animation: animation,
			Document: &domain.Document{ID: 202, MimeType: "application/x-tgsticker", Attributes: []domain.DocumentAttribute{{Kind: domain.DocAttrCustomEmoji, Alt: "🎁", TextColor: true}}, Thumbs: []domain.PhotoSize{{Kind: domain.PhotoSizeKindPath, Type: "j", Bytes: []byte{1}}}},
			Blob:     &domain.FileBlob{LocationKey: "pattern-2"}},
	}
	backdrops := []domain.StarGiftCollectibleAttribute{
		{Kind: domain.StarGiftCollectibleBackdrop, Name: "Backdrop", BackdropID: 0, RarityKind: domain.StarGiftRarityPermille, RarityPermille: 500},
		{Kind: domain.StarGiftCollectibleBackdrop, Name: "Backdrop Two", BackdropID: 1, RarityKind: domain.StarGiftRarityPermille, RarityPermille: 500},
	}
	return domain.StarGiftCollectibleWrite{
		GiftID: giftID, UpgradeStars: 100, SupplyTotal: 1000, SlugPrefix: "sched-test", CommandID: "cmd-1",
		Models: models, Patterns: patterns, Backdrops: backdrops, PublishAt: publishAt,
	}
}

func TestPublishCollectibleRevisionDeferredDropMemory(t *testing.T) {
	ctx := context.Background()
	s := NewStarGiftStore()
	entry, err := s.CreateCatalogRevision(ctx, domain.StarGiftCatalogWrite{Title: "Comet", Stars: 50, ConvertStars: 25, Enabled: true})
	if err != nil {
		t.Fatalf("create catalog gift: %v", err)
	}
	future := time.Now().Add(time.Hour).Unix()
	revision, err := s.PublishCollectibleRevision(ctx, testCollectibleWrite(entry.Gift.ID, future))
	if err != nil {
		t.Fatalf("publish scheduled collectible: %v", err)
	}
	if revision.Published {
		t.Fatalf("future PublishAt should not publish immediately: %+v", revision)
	}
	if revision.ScheduledPublishAt != future {
		t.Fatalf("ScheduledPublishAt=%d, want %d", revision.ScheduledPublishAt, future)
	}

	// The plain gift must stay exactly as not-yet-upgradeable as before the call.
	if _, ok, _ := s.ActiveCollectibleRevision(ctx, entry.Gift.ID); ok {
		t.Fatal("scheduled draft must not be visible as the active/live collectible revision")
	}
	gift, _, _ := s.CatalogGift(ctx, entry.Gift.ID)
	if gift.UpgradeStars != 0 || gift.UpgradeTotal != 0 {
		t.Fatalf("plain gift must stay non-upgradeable pre-drop, got %+v", gift)
	}

	pending, ok, err := s.PendingCollectibleRevision(ctx, entry.Gift.ID)
	if err != nil || !ok {
		t.Fatalf("PendingCollectibleRevision: ok=%v err=%v", ok, err)
	}
	if pending.SupplyTotal != 1000 || pending.ScheduledPublishAt != future {
		t.Fatalf("pending=%+v", pending)
	}

	// Not due yet: activation is a no-op.
	if activated, err := s.ActivateDueCollectibleRevisions(ctx); err != nil || len(activated) != 0 {
		t.Fatalf("premature activation: activated=%v err=%v", activated, err)
	}
}

func TestPublishCollectibleRevisionImmediateMemory(t *testing.T) {
	ctx := context.Background()
	s := NewStarGiftStore()
	entry, err := s.CreateCatalogRevision(ctx, domain.StarGiftCatalogWrite{Title: "Comet", Stars: 50, ConvertStars: 25, Enabled: true})
	if err != nil {
		t.Fatalf("create catalog gift: %v", err)
	}
	revision, err := s.PublishCollectibleRevision(ctx, testCollectibleWrite(entry.Gift.ID, 0))
	if err != nil {
		t.Fatalf("publish immediate collectible: %v", err)
	}
	if !revision.Published {
		t.Fatalf("PublishAt=0 must publish immediately: %+v", revision)
	}
	if _, ok, _ := s.PendingCollectibleRevision(ctx, entry.Gift.ID); ok {
		t.Fatal("an immediately published revision must not also appear as pending")
	}
	gift, _, _ := s.CatalogGift(ctx, entry.Gift.ID)
	if gift.UpgradeStars != 100 || gift.UpgradeTotal != 1000 {
		t.Fatalf("plain gift should reflect the published pool immediately, got %+v", gift)
	}
}

func TestActivateDueCollectibleRevisionsMemory(t *testing.T) {
	ctx := context.Background()
	s := NewStarGiftStore()
	entry, err := s.CreateCatalogRevision(ctx, domain.StarGiftCatalogWrite{Title: "Comet", Stars: 50, ConvertStars: 25, Enabled: true})
	if err != nil {
		t.Fatalf("create catalog gift: %v", err)
	}
	future := time.Now().Add(time.Hour).Unix()
	if _, err := s.PublishCollectibleRevision(ctx, testCollectibleWrite(entry.Gift.ID, future)); err != nil {
		t.Fatalf("publish scheduled collectible: %v", err)
	}
	if _, ok, _ := s.PendingCollectibleRevision(ctx, entry.Gift.ID); !ok {
		t.Fatal("expected a pending scheduled revision before its time arrives")
	}
	// White-box: back-date the schedule to simulate the dispatcher's tick
	// catching up on a drop whose time has now arrived, without depending on
	// wall-clock sleeps in the test.
	s.mu.Lock()
	pending := s.pendingCollectibles[entry.Gift.ID]
	pending.ScheduledPublishAt = time.Now().Add(-time.Minute).Unix()
	s.pendingCollectibles[entry.Gift.ID] = pending
	s.mu.Unlock()

	activated, err := s.ActivateDueCollectibleRevisions(ctx)
	if err != nil {
		t.Fatalf("activate due: %v", err)
	}
	if len(activated) != 1 || activated[0] != entry.Gift.ID {
		t.Fatalf("activated=%v, want [%d]", activated, entry.Gift.ID)
	}
	if _, ok, _ := s.PendingCollectibleRevision(ctx, entry.Gift.ID); ok {
		t.Fatal("revision should no longer be pending once activated")
	}
	if _, ok, _ := s.ActiveCollectibleRevision(ctx, entry.Gift.ID); !ok {
		t.Fatal("revision should now be the active/live collectible revision")
	}
	gift, _, _ := s.CatalogGift(ctx, entry.Gift.ID)
	if gift.UpgradeStars != 100 || gift.UpgradeTotal != 1000 {
		t.Fatalf("gift should now be upgradeable: %+v", gift)
	}
}

// TestCreateCatalogBundleDeferredDropMemory exercises the *official-import*
// path (admin.ImportOfficialStarGift, pulling a real official gift + its
// collectible attributes -- distinct from the manual upload path the other
// tests in this file cover) with a scheduled collectible pool, to make sure
// PublishAt is honored there too, not only via the standalone
// PublishStarGiftCollectibles call.
func TestCreateCatalogBundleDeferredDropMemory(t *testing.T) {
	ctx := context.Background()
	s := NewStarGiftStore()
	future := time.Now().Add(time.Hour).Unix()
	result, err := s.CreateCatalogBundle(ctx, domain.StarGiftCatalogBundleWrite{
		Catalog: domain.StarGiftCatalogWrite{
			Title: "Official Nebula", Stars: 50, ConvertStars: 25, Enabled: true, OfficialGiftID: 5170000000000000001,
		},
		Collectible: func() *domain.StarGiftCollectibleWrite {
			write := testCollectibleWrite(0, future) // GiftID filled in by CreateCatalogBundle
			return &write
		}(),
	})
	if err != nil {
		t.Fatalf("create official catalog bundle with scheduled collectible: %v", err)
	}
	if result.Collectible == nil || result.Collectible.Published {
		t.Fatalf("official import's collectible pool must not publish immediately when scheduled: %+v", result.Collectible)
	}
	giftID := result.Catalog.Gift.ID
	if _, ok, _ := s.ActiveCollectibleRevision(ctx, giftID); ok {
		t.Fatal("official import's scheduled pool must not be active yet")
	}
	if gift, _, _ := s.CatalogGift(ctx, giftID); gift.UpgradeStars != 0 {
		t.Fatalf("official import's plain gift must stay non-upgradeable pre-drop: %+v", gift)
	}
	if _, ok, _ := s.PendingCollectibleRevision(ctx, giftID); !ok {
		t.Fatal("official import's scheduled pool must be visible as pending")
	}
}
