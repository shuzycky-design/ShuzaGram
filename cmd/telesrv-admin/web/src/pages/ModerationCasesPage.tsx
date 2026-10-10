import { ChevronRight, RefreshCw, ShieldAlert } from "lucide-react";
import { useEffect, useState } from "react";
import { api, errorMessage } from "../api";
import { ActionButton } from "../components/ActionButton";
import { Alert, Badge, EmptyRow, Metric, PageFrame, QueryPanel } from "../components/ui";
import { useI18n, type TFunction } from "../i18n";
import { formatDate } from "../lib/format";
import type { Navigate } from "../routing";
import type { ModerationCaseRow, SpamRestrictionSettings } from "../types";

const defaultStatuses = "open,in_review,action_pending,action_failed,appeal_review";
const allStatuses = "open,in_review,action_pending,action_failed,resolved,dismissed,appeal_review";
const statusFilterOptions = [
  { value: defaultStatuses, labelKey: "moderation.statusFilter.active" },
  { value: allStatuses, labelKey: "moderation.statusFilter.all" },
  { value: "open", labelKey: "moderation.status.open" },
  { value: "in_review", labelKey: "moderation.status.in_review" },
  { value: "action_pending", labelKey: "moderation.status.action_pending" },
  { value: "action_failed", labelKey: "moderation.status.action_failed" },
  { value: "appeal_review", labelKey: "moderation.status.appeal_review" },
  { value: "resolved", labelKey: "moderation.status.resolved" },
  { value: "dismissed", labelKey: "moderation.status.dismissed" }
];

export function ModerationCasesPage({ navigate }: { navigate: Navigate }) {
  const { t } = useI18n();
  const [statuses, setStatuses] = useState(defaultStatuses);
  const [assignedTo, setAssignedTo] = useState("");
  const [rows, setRows] = useState<ModerationCaseRow[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [spamSettings, setSpamSettings] = useState<SpamRestrictionSettings | null>(null);
  const [tier1Threshold, setTier1Threshold] = useState("3");
  const [tier2Threshold, setTier2Threshold] = useState("8");
  const [decayHours, setDecayHours] = useState("168");

  async function load() {
    setBusy(true);
    setError("");
    try {
      const params = new URLSearchParams({ statuses, limit: "100" });
      if (assignedTo.trim()) params.set("assigned_to", assignedTo.trim());
      setRows((await api.moderationCases(params)).cases);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function loadSpamSettings() {
    try {
      const settings = await api.spamRestrictionSettings();
      setSpamSettings(settings);
      setTier1Threshold(String(settings.Tier1Threshold));
      setTier2Threshold(String(settings.Tier2Threshold));
      setDecayHours(String(settings.DecayHours));
    } catch (err) {
      setError(errorMessage(err));
    }
  }

  useEffect(() => {
    void load();
    void loadSpamSettings();
  }, []);

  const pendingActions = rows.filter((row) => row.Status === "action_pending" || row.Status === "action_failed").length;
  const critical = rows.filter((row) => row.Severity === 4).length;

  return (
    <PageFrame
      title={t("route.moderation")}
      eyebrow={t("moderation.casesEyebrow")}
      actions={
        <button className="btn icon-text" type="button" onClick={load} disabled={busy}>
          <RefreshCw size={15} className={busy ? "spin" : ""} /> {t("common.refresh")}
        </button>
      }
    >
      {error && <Alert>{error}</Alert>}
      <div className="metric-row">
        <Metric label={t("moderation.currentQueue")} value={String(rows.length)} />
        <Metric label={t("moderation.criticalCases")} value={String(critical)} tone={critical ? "danger" : "neutral"} />
        <Metric label={t("moderation.pendingOrFailed")} value={String(pendingActions)} tone={pendingActions ? "warn" : "good"} />
      </div>
      <div className="query-panel">
        <section className="quick-settings-section">
          <div className="quick-settings-head">
            <strong>{t("moderation.spamSettingsTitle")}</strong>
            <span>{t("moderation.spamSettingsHint")}</span>
          </div>
          <div className="quick-settings-row">
            <label><span>{t("moderation.spamTier1Threshold")}</span><input type="number" min="1" value={tier1Threshold} onChange={(event) => setTier1Threshold(event.target.value)} /></label>
            <label><span>{t("moderation.spamTier2Threshold")}</span><input type="number" min="1" value={tier2Threshold} onChange={(event) => setTier2Threshold(event.target.value)} /></label>
            <label><span>{t("moderation.spamDecayHours")}</span><input type="number" min="1" value={decayHours} onChange={(event) => setDecayHours(event.target.value)} /></label>
            <ActionButton
              compact
              tone="neutral"
              label={t("moderation.saveSpamSettings")}
              path="/api/actions/set-spam-restriction-settings"
              disabled={Number(tier1Threshold) <= 0 || Number(tier2Threshold) <= Number(tier1Threshold) || Number(decayHours) <= 0}
              payload={() => ({
                tier1_threshold: Number(tier1Threshold),
                tier2_threshold: Number(tier2Threshold),
                decay_hours: Number(decayHours)
              })}
              onDone={loadSpamSettings}
            />
          </div>
          {spamSettings && <p className="muted">{t("common.updatedAt")}: {formatDate(spamSettings.UpdatedAt) || "-"}</p>}
        </section>
      </div>
      <QueryPanel>
        <form className="toolbar" onSubmit={(event) => { event.preventDefault(); void load(); }}>
          <label className="field-inline">
            <span>{t("common.status")}</span>
            <select
              aria-label={t("moderation.statusFilter")}
              value={statuses}
              onChange={(event) => setStatuses(event.target.value)}
            >
              {statusFilterOptions.map((option) => (
                <option key={option.value} value={option.value}>{t(option.labelKey)}</option>
              ))}
            </select>
          </label>
          <label className="field-inline">
            <span>{t("moderation.assignee")}</span>
            <input
              value={assignedTo}
              onChange={(event) => setAssignedTo(event.target.value)}
              placeholder={t("moderation.allAssignees")}
            />
          </label>
          <button className="btn primary icon-text" type="submit" disabled={busy}>
            <ShieldAlert size={15} /> {t("common.search")}
          </button>
        </form>
      </QueryPanel>
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>{t("moderation.case")}</th>
              <th>{t("moderation.target")}</th>
              <th>{t("common.status")}</th>
              <th>{t("moderation.severity")}</th>
              <th>{t("moderation.reportsAndReporters")}</th>
              <th>{t("moderation.assignee")}</th>
              <th>{t("moderation.latestReport")}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.ID}>
                <td className="mono">#{row.ID}</td>
                <td className="mono">{moderationTargetLabel(t, row.Target.Type, row.Target.ID)}</td>
                <td><CaseStatus status={row.Status} /></td>
                <td><CaseSeverity value={row.Severity} /></td>
                <td>{row.ReportCount} / {row.DistinctReporterCount}</td>
                <td>{row.AssignedTo || "-"}</td>
                <td>{formatDate(row.LastReportAt)}</td>
                <td>
                  <button className="row-link" onClick={() => navigate(`/moderation/${row.ID}`)}>
                    {t("moderation.review")} <ChevronRight size={14} />
                  </button>
                </td>
              </tr>
            ))}
            {rows.length === 0 && <EmptyRow colSpan={8} />}
          </tbody>
        </table>
      </div>
    </PageFrame>
  );
}

export function CaseStatus({ status }: { status: string }) {
  const { t } = useI18n();
  const tone = status === "resolved" || status === "dismissed"
    ? "good"
    : status === "action_failed"
      ? "danger"
      : status === "action_pending"
        ? "warn"
        : "neutral";
  return <Badge tone={tone}>{moderationEnumLabel(t, "status", status)}</Badge>;
}

export function CaseSeverity({ value }: { value: number }) {
  const { t } = useI18n();
  const keys = ["", "low", "medium", "high", "critical"];
  const key = keys[value];
  return (
    <Badge tone={value >= 4 ? "danger" : value >= 3 ? "warn" : "neutral"}>
      {key ? t(`moderation.severity.${key}`) : value}
    </Badge>
  );
}

export function moderationEnumLabel(t: TFunction, group: string, value: string): string {
  const key = `moderation.${group}.${value}`;
  const translated = t(key);
  return translated === key ? value : translated;
}

export function moderationTargetLabel(t: TFunction, type: string, id: number): string {
  return `${moderationEnumLabel(t, "targetType", type)} #${id}`;
}
