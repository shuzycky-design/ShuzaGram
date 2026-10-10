package rpc

import (
	"context"

	"telesrv/internal/domain"
)

// ensureSpamRestrictionAllowed enforces the sender's current graduated
// spam-report restriction tier (domain.SpamRestriction) against a specific
// private-message recipient. It is independent of the account-freeze gate
// (frozen_gate.go): freeze is a binary, account-wide, read-only state;
// this is a narrower, peer-scoped, three-tier gate restricted to private
// (user-to-user) messaging only -- it never touches groups/channels.
//
//   - SpamRestrictionNone: always allowed.
//   - SpamRestrictionLimited: allowed if the recipient is any contact of the
//     sender, or the sender already has an existing conversation with the
//     recipient (mirrors real Telegram's "can still reply / message
//     contacts, cannot start a new chat with a stranger").
//   - SpamRestrictionSevere: allowed only if the relationship is mutual,
//     with no exception for an existing conversation.
func (r *Router) ensureSpamRestrictionAllowed(ctx context.Context, senderUserID, recipientUserID int64) error {
	if r == nil || r.deps.SpamRestrictions == nil ||
		senderUserID == 0 || recipientUserID == 0 || senderUserID == recipientUserID {
		return nil
	}
	// Built-in service accounts (support, BotFather, @spambot itself, ...)
	// are always reachable regardless of the sender's tier -- otherwise a
	// restricted user could never reach @spambot to find out why, or ask
	// support for help. Real Telegram exempts its own official accounts the
	// same way.
	if domain.IsSystemUserID(recipientUserID) {
		return nil
	}
	restriction, found, err := r.deps.SpamRestrictions.SpamRestriction(ctx, senderUserID)
	if err != nil {
		return internalErr()
	}
	if !found || restriction.Tier == domain.SpamRestrictionNone {
		return nil
	}
	isContact, isMutual, err := r.contactRelationshipForSpamGate(ctx, senderUserID, recipientUserID)
	if err != nil {
		return err
	}
	if restriction.Tier == domain.SpamRestrictionSevere {
		if isMutual {
			return nil
		}
		return spamRestrictedPrivateErr()
	}
	if isContact {
		return nil
	}
	hasExisting, err := r.peerHasExistingConversationForSpamGate(ctx, senderUserID, recipientUserID)
	if err != nil {
		return err
	}
	if hasExisting {
		return nil
	}
	return spamRestrictedPrivateErr()
}

// ensureSpamRestrictionAllowedForGroupSend enforces SpamRestrictionSevere's
// group/channel-posting block: at tier 2 the sender cannot post in any
// group or channel at all, full stop, no exceptions for existing
// membership -- unlike the private-message gate above, there is no
// "mutual contact" equivalent for a group, so the rule is simply blanket.
// Tier 1 never restricts groups (matches the original product spec: only
// the "severe" tier mentions groups at all).
func (r *Router) ensureSpamRestrictionAllowedForGroupSend(ctx context.Context, senderUserID int64) error {
	if r == nil || r.deps.SpamRestrictions == nil || senderUserID == 0 {
		return nil
	}
	restriction, found, err := r.deps.SpamRestrictions.SpamRestriction(ctx, senderUserID)
	if err != nil {
		return internalErr()
	}
	if !found || restriction.Tier != domain.SpamRestrictionSevere {
		return nil
	}
	return spamRestrictedPrivateErr()
}

func (r *Router) contactRelationshipForSpamGate(ctx context.Context, userID, peerUserID int64) (found, mutual bool, err error) {
	if r.deps.Contacts == nil {
		return false, false, nil
	}
	found, mutual, err = r.deps.Contacts.ContactRelationship(ctx, userID, peerUserID)
	if err != nil {
		return false, false, internalErr()
	}
	return found, mutual, nil
}

// peerHasExistingConversationForSpamGate implements tier1's "can still
// reply to anyone who already has a conversation going" exception cheaply:
// if the sender has a private dialog with at least one exchanged message
// for this peer, the conversation predates (or never needed) this check. A
// first unsolicited message from a tier1-restricted sender to a true
// stranger is blocked before any message/dialog row is ever created, so
// this proxy is self-consistently safe.
func (r *Router) peerHasExistingConversationForSpamGate(ctx context.Context, userID, peerUserID int64) (bool, error) {
	if r.deps.Dialogs == nil {
		// Fail-open: never newly block a send because this optional dep is
		// unwired (e.g. a narrow test double).
		return true, nil
	}
	target := domain.Peer{Type: domain.PeerTypeUser, ID: peerUserID}
	list, err := r.deps.Dialogs.GetPeerDialogs(ctx, userID, []domain.Peer{target})
	if err != nil {
		return false, internalErr()
	}
	for _, d := range list.Dialogs {
		if d.Peer == target && d.TopMessage > 0 {
			return true, nil
		}
	}
	return false, nil
}
