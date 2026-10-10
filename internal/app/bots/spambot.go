package bots

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"telesrv/internal/domain"
)

const (
	spamBotHelpText = `I check whether reports from other users have put a restriction on your account, and I message you whenever that changes.

/start - check your current restriction status
/status - check your current restriction status
/help - show this message`
	spamBotUnavailableText  = "Restriction status isn't available right now. Please try again later."
	spamBotUnknownCommand   = "Unknown command. Send /help for available commands."
	spamBotNoneText         = "Good news: there's no restriction on your account right now. You can message anyone freely."
	spamBotLimitedText      = "Your account is currently limited: you can't start a new conversation with someone who isn't your contact and hasn't messaged you first. Replying to people and messaging your own contacts still works as normal.\n\nThis is usually caused by other users reporting your messages."
	spamBotSevereText       = "Your account is currently restricted: you can only exchange private messages with mutual contacts (people who have each other in their contacts), and you can't post in groups or channels. This is a stricter limit than before -- it applies even to conversations you already had.\n\nThis is usually caused by a larger number of reports from other users."
	spamBotLiftedNoticeText = "Your account's messaging restriction has been lifted. You can message anyone freely again."
	spamBotAutoLiftTemplate = "\n\nIt lifts automatically around %s, if no new reports come in before then."
	spamBotManualText       = "\n\nAn administrator set this manually, so it will not lift on its own."
)

// respondAsSpamBot generates and stores @spambot's reply to an incoming
// private message (OnPrivateMessage calls this in a goroutine).
func (s *Service) respondAsSpamBot(userID int64, msg domain.Message) {
	mu := s.serviceBotReplyLock(domain.SpamBotUserID, userID)
	mu.Lock()
	defer mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text := strings.TrimSpace(msg.Body)
	if cmd, ok := parseBotCommand(text); ok {
		switch cmd {
		case "start", "status":
			s.sendServiceBotReply(ctx, domain.SpamBotUserID, userID, s.spamBotStatusReply(ctx, userID))
		case "help":
			s.sendServiceBotReply(ctx, domain.SpamBotUserID, userID, botReply{Text: spamBotHelpText})
		default:
			s.sendServiceBotReply(ctx, domain.SpamBotUserID, userID, botReply{Text: spamBotUnknownCommand})
		}
		return
	}
	// Any other text is treated the same as /status -- real Telegram's own
	// @spambot behaves this way too (there's nothing else to "chat" about).
	s.sendServiceBotReply(ctx, domain.SpamBotUserID, userID, s.spamBotStatusReply(ctx, userID))
}

func (s *Service) spamBotStatusReply(ctx context.Context, userID int64) botReply {
	if s.spamRestrictions == nil {
		return botReply{Text: spamBotUnavailableText}
	}
	restriction, found, err := s.spamRestrictions.SpamRestriction(ctx, userID)
	if err != nil {
		s.log.Warn("spambot: load restriction", zap.Int64("user_id", userID), zap.Error(err))
		return botReply{Text: spamBotUnavailableText}
	}
	if !found || restriction.Tier == domain.SpamRestrictionNone {
		return botReply{Text: spamBotNoneText}
	}
	var eta time.Time
	if !restriction.ManualOverride {
		if settings, err := s.spamRestrictions.SpamRestrictionSettings(ctx); err == nil {
			eta = restriction.LastReportAt.Add(time.Duration(settings.DecayHours) * time.Hour)
		}
	}
	return botReply{Text: spamBotStatusText(restriction.Tier, restriction.ManualOverride, eta)}
}

func spamBotStatusText(tier domain.SpamRestrictionTier, manualOverride bool, autoLiftAt time.Time) string {
	text := spamBotLimitedText
	if tier == domain.SpamRestrictionSevere {
		text = spamBotSevereText
	}
	switch {
	case manualOverride:
		text += spamBotManualText
	case !autoLiftAt.IsZero():
		text += fmt.Sprintf(spamBotAutoLiftTemplate, autoLiftAt.UTC().Format("2006-01-02 15:04 UTC"))
	}
	return text
}

// NotifySpamRestrictionTierChanged proactively messages userID, as
// @spambot, that their restriction tier changed -- called by the RPC
// router's periodic notification sweep
// (ClaimDueSpamRestrictionNotifications), never from the RPC send path
// itself.
func (s *Service) NotifySpamRestrictionTierChanged(ctx context.Context, userID int64, tier domain.SpamRestrictionTier) error {
	if s == nil {
		return fmt.Errorf("bots service is nil")
	}
	text := spamBotLiftedNoticeText
	if tier != domain.SpamRestrictionNone {
		manualOverride := false
		var eta time.Time
		if s.spamRestrictions != nil {
			if restriction, found, err := s.spamRestrictions.SpamRestriction(ctx, userID); err == nil && found {
				manualOverride = restriction.ManualOverride
				if !manualOverride {
					if settings, err := s.spamRestrictions.SpamRestrictionSettings(ctx); err == nil {
						eta = restriction.LastReportAt.Add(time.Duration(settings.DecayHours) * time.Hour)
					}
				}
			}
		}
		text = spamBotStatusText(tier, manualOverride, eta)
	}
	if _, ok := s.sendServiceBotReplyResult(ctx, domain.SpamBotUserID, userID, botReply{Text: text}); !ok {
		return fmt.Errorf("spambot: notify user %d failed", userID)
	}
	return nil
}
