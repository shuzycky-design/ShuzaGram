import { ShieldOff } from "lucide-react";
import { createContext, useContext, useMemo, type ReactNode } from "react";
import { Alert, PageFrame } from "./components/ui";
import { useI18n } from "./i18n";

// Permission names exactly as the backend spells them
// (cmd/telesrv-admin/security.go). "*" is the wildcard an operator configures for
// a full-access session.
export const permissionAll = "*";
export const permissionPremiumManage = "premium.manage";
export const permissionBotTokenRead = "bots.token.read";
export const permissionVerificationReview = "verification.review";
export const permissionVerificationRevoke = "verification.revoke";
// Third-party verification is a separate mechanism and therefore a separate pair of
// rights: review reads the section and decides applications, manage owns the
// verifier roster, the icon catalogue and taking a granted mark away.
export const permissionBotVerificationReview = "botverification.review";
export const permissionBotVerificationManage = "botverification.manage";
// Server Settings: identity, .env, restart/update. One right, not
// review/manage -- see the constant's doc comment in security.go.
export const permissionServerManage = "server.manage";
// Operator accounts. The one right that can hand out every other right, so it
// is never implied by anything else -- see the constant's doc comment in
// security.go. Adapted from github.com/owpengram/owpengram-server (Apache-2.0).
export const permissionAdminsManage = "admins.manage";
// Section rights, in read/manage pairs following the sidebar -- see the const
// block in security.go, which these must match exactly. Adapted from
// owpengram-server alongside permissionAdminsManage; gifts.read/gifts.manage
// is this project's own addition on top of that shape, since gifts are the
// one section owpengram-server does not have.
export const permissionAccountsRead = "accounts.read";
export const permissionAccountsManage = "accounts.manage";
export const permissionChannelsRead = "channels.read";
export const permissionChannelsManage = "channels.manage";
export const permissionBotsRead = "bots.read";
export const permissionBotsManage = "bots.manage";
export const permissionMessagesRead = "messages.read";
export const permissionMessagesManage = "messages.manage";
export const permissionModerationReview = "moderation.review";
export const permissionBroadcastsRead = "broadcasts.read";
export const permissionBroadcastsSend = "broadcasts.send";
export const permissionStorageRead = "storage.read";
export const permissionStorageManage = "storage.manage";
export const permissionContentRead = "content.read";
export const permissionContentManage = "content.manage";
export const permissionUsernamesRead = "usernames.read";
export const permissionUsernamesManage = "usernames.manage";
export const permissionGiftsRead = "gifts.read";
export const permissionGiftsManage = "gifts.manage";
export const permissionDashboardRead = "dashboard.read";

// GET /api/session is read once at boot; the panel keeps the answer here so a
// section the session may not use is hidden instead of rendered into a 403. This
// is a convenience for the operator, not a security boundary: every route is
// checked again server-side.
const PermissionsContext = createContext<readonly string[]>([]);

export function PermissionsProvider({
  permissions,
  children
}: {
  permissions: readonly string[];
  children: ReactNode;
}) {
  return <PermissionsContext.Provider value={permissions}>{children}</PermissionsContext.Provider>;
}

export function usePermissions(): { permissions: readonly string[]; can: (permission: string) => boolean } {
  const permissions = useContext(PermissionsContext);
  return useMemo(
    () => ({
      permissions,
      can: (permission: string) => permissions.includes(permissionAll) || permissions.includes(permission)
    }),
    [permissions]
  );
}

export function useCan(permission: string): boolean {
  return usePermissions().can(permission);
}

// PermissionGate is what a direct URL hits: without the right the operator gets
// an explanation naming the missing permission, not an empty table that looks
// like "no data".
export function PermissionGate({ permission, children }: { permission: string; children: ReactNode }) {
  const { can } = usePermissions();
  if (can(permission)) {
    return <>{children}</>;
  }
  return <PermissionDenied permission={permission} />;
}

export function PermissionDenied({ permission }: { permission: string }) {
  const { t } = useI18n();
  return (
    <PageFrame title={t("permission.deniedTitle")} eyebrow={t("permission.deniedEyebrow")}>
      <Alert>{t("permission.deniedBody", { permission })}</Alert>
      <section className="section-block">
        <div className="entity-head">
          <div>
            <div className="entity-title"><ShieldOff size={16} /> {t("permission.deniedHeading")}</div>
            <div className="entity-subtitle">{t("permission.deniedHint")}</div>
          </div>
        </div>
      </section>
    </PageFrame>
  );
}

// Human-readable names for the permission strings, used by the operator-
// accounts editor (AdminUsersPage). Adapted from github.com/owpengram/
// owpengram-server (Apache-2.0), gifts.* added for this project's own section.
//
// Deliberately not routed through useI18n like the rest of the panel's text:
// these are raw technical permission strings, read mainly by whoever is
// already comfortable enough with the console to be granting rights to other
// operators, and translating twenty-six precise security descriptions across
// three languages is a lot of surface for a screen that sees rare, careful
// use. The raw string is always shown alongside the label (title attribute,
// or in the picker directly), so nothing is hidden behind translation either
// way.
//
// Anything missing from this map falls back to the raw string rather than
// being hidden, so a right added on the server still appears (just
// untranslated) instead of silently vanishing from the editor.
const permissionLabels: Record<string, { title: string; hint: string }> = {
  "accounts.read": { title: "View accounts", hint: "Browse users, their profiles and sessions" },
  "accounts.manage": { title: "Edit accounts", hint: "Change profiles, usernames, freeze and revoke sessions" },
  "channels.read": { title: "View groups and channels", hint: "Browse supergroups and channels" },
  "channels.manage": { title: "Edit groups and channels", hint: "Change settings, usernames and avatars" },
  "bots.read": { title: "View bots", hint: "Browse the bot list and their details" },
  "bots.manage": { title: "Create and delete bots", hint: "Add new bots and remove existing ones" },
  "bots.token.read": { title: "Reveal bot tokens", hint: "Export a bot's live credential" },
  "messages.read": { title: "View messages", hint: "Read private and group message history" },
  "messages.manage": { title: "Delete messages", hint: "Remove messages and clear history" },
  "moderation.review": { title: "Handle reports", hint: "Work the moderation queue and decide cases" },
  "broadcasts.read": { title: "View broadcasts", hint: "See past and scheduled broadcasts" },
  "broadcasts.send": { title: "Send broadcasts", hint: "Deliver a message to many users at once" },
  "content.read": { title: "View stickers, emoji and GIFs", hint: "Browse the packs and the GIF catalogue" },
  "content.manage": { title: "Edit stickers, emoji and GIFs", hint: "Create, rename and remove packs and catalogue entries" },
  "usernames.read": { title: "View NFT usernames", hint: "Browse collectible usernames" },
  "usernames.manage": { title: "Manage NFT usernames", hint: "Mint, transfer and revoke collectible usernames and phones" },
  "gifts.read": { title: "View gifts", hint: "Browse the gift catalogue, auctions and collectibles" },
  "gifts.manage": { title: "Manage gifts", hint: "Import, publish, price, give and delete gifts" },
  "storage.read": { title: "View storage", hint: "See media usage per account" },
  "storage.manage": { title: "Purge storage", hint: "Manually delete stored media" },
  "dashboard.read": { title: "View the dashboard", hint: "See the overview counters and server health" },
  "premium.manage": { title: "Manage Premium", hint: "Grant, revoke and refund Premium" },
  "verification.review": { title: "Verify accounts", hint: "Work the verification queue and grant badges" },
  "verification.revoke": { title: "Remove verification", hint: "Take a granted badge away (needs the right above too)" },
  "botverification.review": { title: "Handle third-party marks", hint: "Work the third-party verification queue" },
  "botverification.manage": { title: "Appoint verifiers", hint: "Grant verifier status and curate mark icons" },
  "server.manage": { title: "Server settings", hint: "Identity, .env editing, restart and update" },
  "admins.manage": { title: "Manage operators", hint: "Create operators and decide what everyone can do" },
  "*": { title: "Full access", hint: "Every right, including future ones" }
};

export function permissionTitle(permission: string): string {
  return permissionLabels[permission]?.title ?? permission;
}

export function permissionHint(permission: string): string {
  return permissionLabels[permission]?.hint ?? "";
}

// Rights grouped by the part of the console they govern, so the editor reads
// as a few short decisions instead of one wall of checkboxes. Adapted from
// owpengram-server; the Gifts group is this project's own addition.
//
// Order is roughly "everyday work first, keys to the building last": an
// operator scanning down the list meets the routine rights before the ones
// that can undo the deployment.
export const permissionGroups: { title: string; hint: string; permissions: string[] }[] = [
  {
    title: "People and chats",
    hint: "Users, groups and their message history",
    permissions: ["accounts.read", "accounts.manage", "channels.read", "channels.manage", "messages.read", "messages.manage"]
  },
  {
    title: "Moderation and verification",
    hint: "Reports, badges and third-party marks",
    permissions: ["moderation.review", "verification.review", "verification.revoke", "botverification.review", "botverification.manage"]
  },
  {
    title: "Content",
    hint: "Sticker packs, emoji, GIFs and collectible usernames",
    permissions: ["content.read", "content.manage", "usernames.read", "usernames.manage"]
  },
  {
    title: "Gifts",
    hint: "The gift catalogue, auctions and collectibles",
    permissions: ["gifts.read", "gifts.manage"]
  },
  {
    title: "Bots",
    hint: "The bot roster and its credentials",
    permissions: ["bots.read", "bots.manage", "bots.token.read"]
  },
  {
    title: "Broadcasting",
    hint: "Messages sent to many users at once",
    permissions: ["broadcasts.read", "broadcasts.send"]
  },
  {
    title: "Storage and overview",
    hint: "Media usage and the dashboard",
    permissions: ["storage.read", "storage.manage", "dashboard.read"]
  },
  {
    title: "Billing",
    hint: "Premium grants and refunds",
    permissions: ["premium.manage"]
  },
  {
    title: "The console itself",
    hint: "The two rights that can change the deployment or hand out every other right",
    permissions: ["server.manage", "admins.manage"]
  }
];

// groupPermissions arranges the server's list into the groups above. Anything
// the server offers that no group claims is collected at the end rather than
// dropped, so a right added on the backend still appears here without this
// file having to be edited first.
export function groupPermissions(available: string[]): { title: string; hint: string; permissions: string[] }[] {
  const remaining = new Set(available);
  const out: { title: string; hint: string; permissions: string[] }[] = [];
  for (const group of permissionGroups) {
    const present = group.permissions.filter((p) => remaining.has(p));
    present.forEach((p) => remaining.delete(p));
    if (present.length > 0) {
      out.push({ title: group.title, hint: group.hint, permissions: present });
    }
  }
  if (remaining.size > 0) {
    out.push({ title: "Other", hint: "Rights this console version does not have a group for", permissions: [...remaining] });
  }
  return out;
}
