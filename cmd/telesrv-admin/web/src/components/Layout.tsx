import {
  AtSign,
  BadgeCheck,
  Bot,
  ChevronDown,
  Database,
  Gavel,
	Film,
  Globe2,
  LayoutDashboard,
  LogOut,
  Menu,
  MessageSquareText,
  Megaphone,
  Phone,
  BadgeDollarSign,
  Rss,
  Server,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Smile,
  Stamp,
  Sticker,
  Trophy,
  Users,
  UserCog,
	Gift,
	Send
} from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { api } from "../api";
import { LanguageSwitch, useI18n } from "../i18n";
import {
  permissionAdminsManage,
  permissionBotVerificationReview,
  permissionPremiumManage,
  permissionVerificationReview,
  useCan
} from "../permissions";
import { type Navigate, type RouteState, routeSubtitle, routeTitle } from "../routing";
import { ThemeSwitch } from "../theme";
import { AppLink } from "./AppLink";

export function BootScreen() {
  const { t } = useI18n();
  return (
    <div className="boot-screen">
      <div className="brand compact brand-elevated">
        <img className="brand-mark" src="/brand-mark.jpg" alt="ShuzaGram" />
        <span>
          <strong>ShuzaGram</strong>
          <small>{t("app.adminConsole")}</small>
        </span>
      </div>
      <div className="loader-bar" />
    </div>
  );
}

export function Shell({
  actor,
  route,
  navigate,
  onLogout,
  children
}: {
  actor: string;
  route: RouteState;
  navigate: Navigate;
  onLogout: () => void;
  children: ReactNode;
}) {
  const { t } = useI18n();
  // The verification queue is hidden for a session without verification.review:
  // the entry would only lead to a 403 (and the route itself is gated as well).
  const canReviewVerification = useCan(permissionVerificationReview);
  // Same reasoning for the third-party queue, which has its own right: the two
  // sections are granted independently, so one entry can be visible without the other.
  const canReviewBotVerification = useCan(permissionBotVerificationReview);
  const canManagePremium = useCan(permissionPremiumManage);
  // Hidden for the same reason as the verification entries above: without
  // admins.manage the link only leads to a 403 (and the route is gated too).
  const canManageAdmins = useCan(permissionAdminsManage);
  // The sidebar is an off-canvas drawer, closed by default: the content
  // column gets the full width and the ~25-destination nav only appears
  // when the operator actually asks for it via the topbar's menu button,
  // instead of permanently occupying a fixed-width column on every page.
  const [sidebarOpen, setSidebarOpen] = useState(false);
  useEffect(() => {
    setSidebarOpen(false);
  }, [route.path]);
  const messagesActive = route.path.startsWith("/messages");
  // One open/closed flag per nav group (keyed by its label), all defaulting
  // open: grouping existing items under headings is about giving the ~25-item
  // flat list some structure, not about hiding destinations by default --
  // collapsing is an option the operator reaches for, not a surprise.
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({});
  function isGroupOpen(key: string) {
    return openGroups[key] ?? true;
  }
  function toggleGroup(key: string) {
    setOpenGroups((prev) => ({ ...prev, [key]: !(prev[key] ?? true) }));
  }
  const [messagesOpen, setMessagesOpen] = useState(messagesActive);

  useEffect(() => {
    if (messagesActive) {
      setMessagesOpen(true);
    }
  }, [messagesActive]);

  useEffect(() => {
    if (!sidebarOpen) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setSidebarOpen(false);
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [sidebarOpen]);

  async function logout() {
    await api.logout().catch(() => undefined);
    onLogout();
  }

  return (
    <div className="shell">
      {sidebarOpen && <div className="sidebar-backdrop" onClick={() => setSidebarOpen(false)} aria-hidden="true" />}
      <aside className={`sidebar ${sidebarOpen ? "open" : ""}`}>
        <AppLink className="brand" href="/" navigate={navigate}>
          <img className="brand-mark" src="/brand-mark.jpg" alt="ShuzaGram" />
          <span>
            <strong>ShuzaGram</strong>
            <small>{t("app.adminConsole")}</small>
          </span>
        </AppLink>
        <div className="sidebar-label">{t("layout.navigation")}</div>
        <nav className="nav-list" aria-label={t("layout.primaryNav")}>
          <NavLink icon={<LayoutDashboard size={16} />} href="/" route={route} navigate={navigate}>{t("layout.dashboard")}</NavLink>

          <NavGroup groupKey="people" label={t("layout.groupPeople")} icon={<ShieldCheck size={16} />} open={isGroupOpen("people")} onToggle={() => toggleGroup("people")}>
            <NavLink icon={<Users size={16} />} href="/accounts" route={route} navigate={navigate}>{t("layout.accounts")}</NavLink>
            <NavLink icon={<ShieldCheck size={16} />} href="/channels" route={route} navigate={navigate}>{t("layout.channels")}</NavLink>
            <NavLink icon={<ShieldAlert size={16} />} href="/moderation" route={route} navigate={navigate}>{t("layout.moderation")}</NavLink>
            {canReviewVerification && (
              <NavLink icon={<BadgeCheck size={16} />} href="/verification" route={route} navigate={navigate}>{t("layout.verification")}</NavLink>
            )}
            {canReviewBotVerification && (
              <NavLink icon={<Stamp size={16} />} href="/bot-verification" route={route} navigate={navigate}>{t("layout.botVerification")}</NavLink>
            )}
            <NavLink icon={<Trophy size={16} />} href="/account-ratings" route={route} navigate={navigate}>{t("layout.accountRatings")}</NavLink>
          </NavGroup>

          <NavGroup groupKey="messaging" label={t("layout.groupMessaging")} icon={<MessageSquareText size={16} />} open={isGroupOpen("messaging")} onToggle={() => toggleGroup("messaging")}>
            <NavLink icon={<Bot size={16} />} href="/bots" route={route} navigate={navigate}>{t("layout.bots")}</NavLink>
            <NavLink icon={<Megaphone size={16} />} href="/broadcasts" route={route} navigate={navigate}>{t("layout.broadcasts")}</NavLink>
            <NavLink icon={<Rss size={16} />} href="/auto-subscribe-channels" route={route} navigate={navigate}>{t("layout.autoSubscribeChannels")}</NavLink>
            <div className={`nav-section ${messagesActive ? "active" : ""} ${messagesOpen ? "open" : ""}`}>
              <button
                className="nav-section-toggle"
                type="button"
                aria-expanded={messagesOpen}
                onClick={() => setMessagesOpen((open) => !open)}
              >
                <MessageSquareText size={16} />
                <span>{t("layout.messages")}</span>
                <ChevronDown className="nav-section-chevron" size={15} />
              </button>
              {messagesOpen && (
                <div className="nav-children">
                  <NavLink
                    href="/messages/private"
                    route={route}
                    navigate={navigate}
                    activeWhen={(path) => path === "/messages" || path === "/messages/detail" || path.startsWith("/messages/private")}
                  >
                    {t("layout.privateMessages")}
                  </NavLink>
                  <NavLink
                    href="/messages/groups"
                    route={route}
                    navigate={navigate}
                    activeWhen={(path) => path.startsWith("/messages/groups")}
                  >
                    {t("layout.groupMessages")}
                  </NavLink>
                </div>
              )}
            </div>
          </NavGroup>

          <NavGroup groupKey="commerce" label={t("layout.groupCommerce")} icon={<Gift size={16} />} open={isGroupOpen("commerce")} onToggle={() => toggleGroup("commerce")}>
            {canManagePremium && (
              <NavLink icon={<BadgeDollarSign size={16} />} href="/monetization" route={route} navigate={navigate}
                activeWhen={(path) => path.startsWith("/monetization") || path.startsWith("/premium")}>
                {t("layout.premium")}
              </NavLink>
            )}
            <NavLink icon={<Gift size={16} />} href="/gifts" route={route} navigate={navigate}>{t("layout.gifts")}</NavLink>
            <NavLink icon={<Send size={16} />} href="/give-gifts" route={route} navigate={navigate}>{t("layout.giveGifts")}</NavLink>
            <NavLink icon={<Gavel size={16} />} href="/auctions" route={route} navigate={navigate}>{t("layout.auctions")}</NavLink>
            <NavLink icon={<AtSign size={16} />} href="/collectible-usernames" route={route} navigate={navigate}>{t("layout.collectibleUsernames")}</NavLink>
            <NavLink icon={<Phone size={16} />} href="/collectible-phones" route={route} navigate={navigate}>{t("layout.collectiblePhones")}</NavLink>
          </NavGroup>

          <NavGroup groupKey="content" label={t("layout.groupContent")} icon={<Sticker size={16} />} open={isGroupOpen("content")} onToggle={() => toggleGroup("content")}>
            <NavLink icon={<Sticker size={16} />} href="/stickers" route={route} navigate={navigate}>{t("layout.stickers")}</NavLink>
            <NavLink icon={<Smile size={16} />} href="/emoji" route={route} navigate={navigate}>{t("layout.emoji")}</NavLink>
            <NavLink icon={<Film size={16} />} href="/gif-catalog" route={route} navigate={navigate}>{t("layout.gifCatalog")}</NavLink>
          </NavGroup>

          <NavGroup groupKey="platform" label={t("layout.groupPlatform")} icon={<Server size={16} />} open={isGroupOpen("platform")} onToggle={() => toggleGroup("platform")}>
            <NavLink icon={<Globe2 size={16} />} href="/countries" route={route} navigate={navigate}>{t("layout.countries")}</NavLink>
            <NavLink icon={<Database size={16} />} href="/storage" route={route} navigate={navigate}>{t("layout.storage")}</NavLink>
            {canManageAdmins && (
              <NavLink icon={<UserCog size={16} />} href="/admin-users" route={route} navigate={navigate}>{t("layout.adminUsers")}</NavLink>
            )}
          </NavGroup>
        </nav>
        <div className="sidebar-status">
          <div className="sidebar-label">{t("layout.runtime")}</div>
          <div className="runtime-row"><Server size={14} /><span>{t("layout.adminBackend")}</span><strong>{t("layout.ready")}</strong></div>
          <div className="runtime-row"><Database size={14} /><span>{t("layout.pgRead")}</span><strong>{t("layout.readOnly")}</strong></div>
          <div className="runtime-row"><Shield size={14} /><span>{t("layout.writeOps")}</span><strong>{t("layout.dryRun")}</strong></div>
        </div>
      </aside>
      <div className="workspace">
        <header className="topbar">
          <div className="topbar-start">
            <button
              className="menu-toggle"
              type="button"
              aria-label={t("layout.toggleNavigation")}
              aria-expanded={sidebarOpen}
              onClick={() => setSidebarOpen((open) => !open)}
            >
              <Menu size={18} />
            </button>
            <div>
              <div className="eyebrow">{routeSubtitle(route.path, t)}</div>
              <h1>{routeTitle(route.path, t)}</h1>
            </div>
          </div>
          <div className="topbar-actions">
            <ThemeSwitch />
            <LanguageSwitch />
            <span className="actor-pill">{t("layout.actor", { actor })}</span>
            <button className="btn ghost icon-text" type="button" onClick={logout} title={t("layout.logout")}>
              <LogOut size={16} /> {t("layout.logout")}
            </button>
          </div>
        </header>
        <main className="content">{children}</main>
      </div>
    </div>
  );
}

// NavGroup is the generalized form of the one-off "Messages" collapsible
// section: a labeled heading over a handful of related destinations, so the
// sidebar reads as a handful of named areas instead of one long flat list of
// ~25 links. Defaults open (see isGroupOpen in Shell) -- grouping is about
// giving the list structure, not about hiding things by default.
function NavGroup({
  label,
  icon,
  open,
  onToggle,
  children
}: {
  groupKey: string;
  label: string;
  icon: ReactNode;
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  return (
    <div className={`nav-group ${open ? "open" : ""}`}>
      <button className="nav-group-toggle" type="button" aria-expanded={open} onClick={onToggle}>
        {icon}
        <span>{label}</span>
        <ChevronDown className="nav-section-chevron" size={14} />
      </button>
      {open && <div className="nav-group-children">{children}</div>}
    </div>
  );
}

function NavLink({
  href,
  route,
  navigate,
  icon,
  children,
  activeWhen
}: {
  href: string;
  route: RouteState;
  navigate: Navigate;
  icon?: ReactNode;
  children: ReactNode;
  activeWhen?: (path: string) => boolean;
}) {
  const active = activeWhen ? activeWhen(route.path) : href === "/" ? route.path === "/" : route.path.startsWith(href);
  return (
    <AppLink className={`nav-item ${active ? "active" : ""}`} href={href} navigate={navigate}>
      {icon ?? <span aria-hidden="true" className="nav-dot" />}
      <span>{children}</span>
    </AppLink>
  );
}
