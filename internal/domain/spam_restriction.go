package domain

import "time"

// SpamRestrictionTier is the graduated anti-spam private-messaging
// restriction level automatically derived from distinct report counts
// against a user (see SpamRestriction), independent of AccountFreeze.
type SpamRestrictionTier int16

const (
	// SpamRestrictionNone is the default, unrestricted state.
	SpamRestrictionNone SpamRestrictionTier = 0
	// SpamRestrictionLimited blocks starting a new private conversation with
	// a stranger (not a contact, no existing conversation) but still allows
	// replying to anyone and messaging contacts freely.
	SpamRestrictionLimited SpamRestrictionTier = 1
	// SpamRestrictionSevere restricts private messaging to mutual contacts
	// only, with no exception for an existing conversation. It never affects
	// posting in groups/channels the user is already a member of.
	SpamRestrictionSevere SpamRestrictionTier = 2
)

// SpamRestriction is one user's current restriction state.
type SpamRestriction struct {
	UserID                int64
	Tier                  SpamRestrictionTier
	CaseID                int64
	DistinctReporterCount int
	LastReportAt          time.Time
	// ManualOverride, once set by an admin action, pins Tier and excludes the
	// row from both automatic escalation (the moderation report hook only
	// raises tier on rows where NOT ManualOverride) and the decay sweep.
	ManualOverride bool
	Actor          string
	Reason         string
	Version        int64
	UpdatedAt      time.Time
}

// SpamRestrictionSettings are the admin-tunable global thresholds and decay
// window driving automatic tier escalation and auto-lift.
type SpamRestrictionSettings struct {
	Tier1Threshold int
	Tier2Threshold int
	DecayHours     int
	UpdatedAt      time.Time
	UpdatedBy      string
}

// SpamRestrictionNotification is one pending "@spambot should tell this user
// their tier changed" claim, returned by
// SpamRestrictionStore.ClaimDueSpamRestrictionNotifications.
type SpamRestrictionNotification struct {
	UserID int64
	Tier   SpamRestrictionTier
}
