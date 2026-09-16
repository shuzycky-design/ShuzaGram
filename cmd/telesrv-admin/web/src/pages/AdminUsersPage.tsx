import { KeyRound, Lock, RefreshCw, ShieldCheck, Trash2, UserPlus, X } from "lucide-react";
import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { api, errorMessage } from "../api";
import { ActionButton } from "../components/ActionButton";
import { Alert, Badge, EmptyRow, PageFrame, QueryPanel, SectionHead } from "../components/ui";
import { useI18n } from "../i18n";
import { groupPermissions, permissionAll, permissionHint, permissionTitle } from "../permissions";
import type { AdminConsoleSystemOperator, AdminConsoleUser } from "../types";

// The operator-accounts screen. The table only reports; every change happens in
// a modal and goes through the panel's usual reason + dry-run + confirm flow,
// because handing somebody the run of the console deserves the same "here is
// what this will do" step as freezing an account.
//
// Everything here is additionally enforced server-side by admins.manage --
// hiding the section is a convenience, not the boundary.
//
// Adapted from github.com/owpengram/owpengram-server (Apache-2.0) -- see
// cmd/telesrv-admin/adminusers.go's package doc comment. The delete flow
// below is this project's own addition; owpengram-server has no equivalent
// button.
export function AdminUsersPage() {
  const { t } = useI18n();
  const [rows, setRows] = useState<AdminConsoleUser[]>([]);
  const [system, setSystem] = useState<AdminConsoleSystemOperator | null>(null);
  const [available, setAvailable] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState<AdminConsoleUser | null>(null);
  const [resetting, setResetting] = useState<AdminConsoleUser | null>(null);
  const [deleting, setDeleting] = useState<AdminConsoleUser | null>(null);
  const [creating, setCreating] = useState(false);

  async function load() {
    setBusy(true);
    setError("");
    try {
      const result = await api.adminUsers();
      setRows(result.rows ?? []);
      setSystem(result.system ?? null);
      setAvailable(result.available_permissions ?? []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  return (
    <PageFrame eyebrow={t("adminUsers.eyebrow")} title={t("adminUsers.pageTitle")}>
      {error && <Alert>{error}</Alert>}

      <QueryPanel>
        <div className="toolbar">
          <button className="btn primary icon-text" type="button" onClick={() => setCreating(true)}>
            <UserPlus size={15} /> {t("adminUsers.newOperator")}
          </button>
          <button className="btn icon-text" type="button" onClick={() => void load()} disabled={busy}>
            <RefreshCw size={15} className={busy ? "spin" : ""} /> {t("common.refresh")}
          </button>
        </div>
      </QueryPanel>

      <SectionHead title={t("adminUsers.sectionTitle")} />
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>{t("adminUsers.colUsername")}</th>
              <th>{t("adminUsers.colCanDo")}</th>
              <th>{t("adminUsers.colStatus")}</th>
              <th>{t("adminUsers.colLastLogin")}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {/* The built-in operator first: it has the most rights and no
                database row, so a list that started with the named accounts
                would put the most powerful login last, or nowhere. */}
            {system && (
              <tr>
                <td className="mono">
                  {system.username} <Badge>{t("adminUsers.builtIn")}</Badge>
                </td>
                <td><PermissionChips permissions={system.permissions} /></td>
                <td><Badge tone="good">{t("adminUsers.enabled")}</Badge></td>
                <td className="mono">{"—"}</td>
                <td>
                  <span className="muted icon-text">
                    <Lock size={13} /> {t("adminUsers.setInEnv")}
                  </span>
                </td>
              </tr>
            )}
            {rows.map((row) => (
              <tr key={row.id}>
                <td className="mono">{row.username}</td>
                <td><PermissionChips permissions={row.permissions} /></td>
                <td>
                  {row.enabled
                    ? <Badge tone="good">{t("adminUsers.enabled")}</Badge>
                    : <Badge>{t("adminUsers.disabled")}</Badge>}
                </td>
                <td className="mono">{row.last_login_at ? new Date(row.last_login_at).toLocaleString() : "—"}</td>
                <td>
                  <div className="toolbar">
                    <button className="btn icon-text" type="button" onClick={() => setEditing(row)}>
                      <ShieldCheck size={14} /> {t("adminUsers.access")}
                    </button>
                    <button className="btn icon-text" type="button" onClick={() => setResetting(row)}>
                      <KeyRound size={14} /> {t("adminUsers.password")}
                    </button>
                    <button className="btn icon-text danger" type="button" onClick={() => setDeleting(row)}>
                      <Trash2 size={14} /> {t("adminUsers.delete")}
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {rows.length === 0 && !system && <EmptyRow colSpan={5} />}
          </tbody>
        </table>
      </div>

      {creating && (
        <OperatorModal
          title={t("adminUsers.newOperatorTitle")}
          available={available}
          onClose={() => setCreating(false)}
          onDone={() => { setCreating(false); void load(); }}
        />
      )}
      {editing && (
        <OperatorModal
          title={t("adminUsers.accessForTitle", { username: editing.username })}
          available={available}
          existing={editing}
          onClose={() => setEditing(null)}
          onDone={() => { setEditing(null); void load(); }}
        />
      )}
      {resetting && (
        <PasswordModal
          operator={resetting}
          onClose={() => setResetting(null)}
          onDone={() => { setResetting(null); void load(); }}
        />
      )}
      {deleting && (
        <DeleteModal
          operator={deleting}
          onClose={() => setDeleting(null)}
          onDone={() => { setDeleting(null); void load(); }}
        />
      )}
    </PageFrame>
  );
}

function PermissionChips({ permissions }: { permissions: string[] }) {
  const { t } = useI18n();
  if (permissions.length === 0) {
    return <span className="muted">{t("adminUsers.nothingYet")}</span>;
  }
  return (
    <span className="chip-row">
      {permissions.map((p) => (
        <span key={p} title={p}><Badge>{permissionTitle(p)}</Badge></span>
      ))}
    </span>
  );
}

// PermissionPicker lists the rights by what they let someone do, split into the
// section of the console each governs -- twenty-six checkboxes in one run is a
// wall nobody reads, and the grouping is what makes "what can this person
// actually touch" answerable at a glance.
//
// The raw permission string stays as each row's tooltip, so the screen never
// hides what is actually being stored.
//
// "*" gets its own row rather than a box in the grid below, because
// assignablePermissions() deliberately leaves it out of the assignable list
// (cmd/telesrv-admin/security.go) -- without this row an operator holding the
// wildcard, like one granted through TELESRV_ADMIN_UI_PERMISSIONS, renders as
// every box unticked while Has() answers true for everything, and there is no
// way to take it away again. While it is on the grid is disabled:
// normalisePermissions collapses "*" plus anything back to just "*", so
// ticking a box there would be a no-op the screen would otherwise show as a
// change.
function PermissionPicker({
  available,
  selected,
  onToggle,
  onToggleGroup,
  onToggleAll
}: {
  available: string[];
  selected: string[];
  onToggle: (permission: string, on: boolean) => void;
  onToggleGroup: (permissions: string[], on: boolean) => void;
  onToggleAll: (on: boolean) => void;
}) {
  const { t } = useI18n();
  const full = selected.includes(permissionAll);
  return (
    <div className="permission-groups">
      <section className="permission-group">
        <div className="permission-grid">
          <label className="permission-item" title={permissionAll}>
            <input type="checkbox" checked={full} onChange={(event) => onToggleAll(event.target.checked)} />
            <span className="permission-copy">
              <strong>{permissionTitle(permissionAll)}</strong>
              <small>{permissionHint(permissionAll)}</small>
            </span>
          </label>
        </div>
      </section>
      {groupPermissions(available).map((group) => {
        const all = group.permissions.every((p) => selected.includes(p));
        return (
          <section className="permission-group" key={group.title}>
            <div className="permission-group-head">
              <div>
                <strong>{group.title}</strong>
                <small>{group.hint}</small>
              </div>
              <button
                className="btn compact"
                type="button"
                disabled={full}
                onClick={() => onToggleGroup(group.permissions, !all)}
              >
                {all ? t("adminUsers.clear") : t("adminUsers.selectAll")}
              </button>
            </div>
            <div className="permission-grid">
              {group.permissions.map((permission) => (
                <label className="permission-item" key={permission} title={permission}>
                  <input
                    type="checkbox"
                    checked={full || selected.includes(permission)}
                    disabled={full}
                    onChange={(event) => onToggle(permission, event.target.checked)}
                  />
                  <span className="permission-copy">
                    <strong>{permissionTitle(permission)}</strong>
                    <small>{permissionHint(permission)}</small>
                  </span>
                </label>
              ))}
            </div>
          </section>
        );
      })}
    </div>
  );
}

// OperatorModal creates a new operator, or edits an existing one's access. The
// same shape either way: the only difference is whether a username and password
// are being chosen.
//
// Laid out as head / scrolling body / action bar like every other command modal
// in the panel, so a long permission list scrolls inside the dialog instead of
// pushing its own confirm button off the screen.
function OperatorModal({
  title,
  available,
  existing,
  onClose,
  onDone
}: {
  title: string;
  available: string[];
  existing?: AdminConsoleUser;
  onClose: () => void;
  onDone: () => void;
}) {
  const { t } = useI18n();
  const [username, setUsername] = useState(existing?.username ?? "");
  const [password, setPassword] = useState("");
  const [permissions, setPermissions] = useState<string[]>(existing?.permissions ?? []);
  const [enabled, setEnabled] = useState(existing?.enabled ?? true);
  const isEdit = Boolean(existing);

  // Only the shape the server insists on: a username it will accept, and a
  // password that is actually present. Length is the operator's business.
  const incomplete = isEdit
    ? false
    : username.trim().length < 3 || password.trim() === "";

  return createPortal(
    <div className="modal-backdrop" role="presentation">
      <section className="modal command-modal" role="dialog" aria-modal="true" aria-label={title}>
        <div className="modal-head">
          <div>
            <div className="eyebrow">{t("adminUsers.sectionTitle")}</div>
            <h2>{title}</h2>
          </div>
          <button className="icon-btn" type="button" onClick={onClose} aria-label={t("action.close")}><X size={15} /></button>
        </div>

        <div className="command-body">
          {!isEdit && (
            <div className="operator-identity">
              <label className="duration-field">
                <span>{t("adminUsers.username")}</span>
                <input
                  autoFocus
                  value={username}
                  spellCheck={false}
                  autoCapitalize="none"
                  placeholder={t("adminUsers.usernameHint")}
                  onChange={(event) => setUsername(event.target.value)}
                />
              </label>
              <label className="duration-field">
                <span>{t("adminUsers.passwordLabel")}</span>
                <input
                  type="password"
                  value={password}
                  autoComplete="new-password"
                  onChange={(event) => setPassword(event.target.value)}
                />
              </label>
            </div>
          )}

          <PermissionPicker
            available={available}
            selected={permissions}
            onToggle={(permission, on) =>
              setPermissions((current) =>
                on ? [...current, permission] : current.filter((p) => p !== permission)
              )
            }
            onToggleGroup={(group, on) =>
              setPermissions((current) =>
                on
                  ? [...current, ...group.filter((p) => !current.includes(p))]
                  : current.filter((p) => !group.includes(p))
              )
            }
            onToggleAll={(on) =>
              setPermissions((current) =>
                on ? [permissionAll] : current.filter((p) => p !== permissionAll)
              )
            }
          />

          <label className="permission-item standalone">
            <input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} />
            <span className="permission-copy">
              <strong>{t("adminUsers.accountEnabled")}</strong>
              <small>{t("adminUsers.accountEnabledHint")}</small>
            </span>
          </label>

          {isEdit && <Alert>{t("adminUsers.editNotice")}</Alert>}
        </div>

        <div className="modal-actions toolbar">
          <button className="btn" type="button" onClick={onClose}>{t("adminUsers.cancel")}</button>
          <ActionButton
            label={isEdit ? t("adminUsers.saveAccess") : t("adminUsers.createOperator")}
            path={isEdit ? "/api/actions/set-admin-user-access" : "/api/actions/create-admin-user"}
            tone="neutral"
            disabled={incomplete}
            icon={isEdit ? <ShieldCheck size={15} /> : <UserPlus size={15} />}
            payload={() =>
              isEdit
                ? { id: existing?.id, permissions, enabled }
                : { username: username.trim(), password, permissions, enabled }
            }
            onDone={onDone}
          />
        </div>
      </section>
    </div>,
    document.body
  );
}

function PasswordModal({
  operator,
  onClose,
  onDone
}: {
  operator: AdminConsoleUser;
  onClose: () => void;
  onDone: () => void;
}) {
  const { t } = useI18n();
  const [password, setPassword] = useState("");

  return createPortal(
    <div className="modal-backdrop" role="presentation">
      <section className="modal command-modal narrow" role="dialog" aria-modal="true" aria-label={t("adminUsers.passwordTitle", { username: operator.username })}>
        <div className="modal-head">
          <div>
            <div className="eyebrow">{t("adminUsers.sectionTitle")}</div>
            <h2>{t("adminUsers.passwordTitle", { username: operator.username })}</h2>
          </div>
          <button className="icon-btn" type="button" onClick={onClose} aria-label={t("action.close")}><X size={15} /></button>
        </div>

        <div className="command-body">
          <label className="duration-field">
            <span>{t("adminUsers.newPassword")}</span>
            <input
              autoFocus
              type="password"
              value={password}
              autoComplete="new-password"
              onChange={(event) => setPassword(event.target.value)}
            />
          </label>
          <Alert>{t("adminUsers.passwordNotice")}</Alert>
        </div>

        <div className="modal-actions toolbar">
          <button className="btn" type="button" onClick={onClose}>{t("adminUsers.cancel")}</button>
          <ActionButton
            label={t("adminUsers.setPassword")}
            path="/api/actions/set-admin-user-password"
            tone="warn"
            disabled={password.trim() === ""}
            icon={<KeyRound size={15} />}
            payload={() => ({ id: operator.id, password })}
            onDone={onDone}
          />
        </div>
      </section>
    </div>,
    document.body
  );
}

// DeleteModal is this project's own addition on top of the ported owpengram-
// server screen: the backend already exposes delete-admin-user (see
// adminusers_api.go's handleDeleteAdminUserAPI), guarded by the same last-
// manager check as disabling an account, so the panel should let an operator
// actually reach it rather than leaving it curl-only.
function DeleteModal({
  operator,
  onClose,
  onDone
}: {
  operator: AdminConsoleUser;
  onClose: () => void;
  onDone: () => void;
}) {
  const { t } = useI18n();
  return createPortal(
    <div className="modal-backdrop" role="presentation">
      <section className="modal command-modal narrow" role="dialog" aria-modal="true" aria-label={t("adminUsers.deleteTitle", { username: operator.username })}>
        <div className="modal-head">
          <div>
            <div className="eyebrow">{t("adminUsers.sectionTitle")}</div>
            <h2>{t("adminUsers.deleteTitle", { username: operator.username })}</h2>
          </div>
          <button className="icon-btn" type="button" onClick={onClose} aria-label={t("action.close")}><X size={15} /></button>
        </div>

        <div className="command-body">
          <Alert>{t("adminUsers.deleteWarning")}</Alert>
        </div>

        <div className="modal-actions toolbar">
          <button className="btn" type="button" onClick={onClose}>{t("adminUsers.cancel")}</button>
          <ActionButton
            label={t("adminUsers.deleteConfirm")}
            path="/api/actions/delete-admin-user"
            tone="danger"
            icon={<Trash2 size={15} />}
            payload={() => ({ id: operator.id })}
            onDone={onDone}
          />
        </div>
      </section>
    </div>,
    document.body
  );
}
