import { Trash2, X } from "lucide-react";
import { useState } from "react";
import { createPortal } from "react-dom";
import { ActionButton } from "../components/ActionButton";
import { useI18n } from "../i18n";
import type { StarGiftRow } from "../types";

// Quick per-gift admin controls, reachable directly from the catalog row
// instead of the full "Replace" revision form: sort order, the plain gift's
// own "X of Y sold" cap, and permanent deletion. Each section is its own
// ActionButton (reason -> dry-run -> confirm, same as every other admin
// action in this panel) rather than one combined submit, so an operator can
// tweak just the one thing they came for.
export function GiftQuickSettingsModal({ gift, onClose, onChanged }: { gift: StarGiftRow; onClose: () => void; onChanged: () => void }) {
  const { t } = useI18n();
  const [sortOrder, setSortOrder] = useState(String(gift.SortOrder));
  const [limited, setLimited] = useState(gift.Limited);
  const [total, setTotal] = useState(String(gift.AvailabilityTotal || 0));
  const [issued, setIssued] = useState(String(gift.Limited ? gift.AvailabilityTotal - gift.AvailabilityRemains : 0));
  const [refundStars, setRefundStars] = useState(false);

  return createPortal(<div className="modal-backdrop" role="presentation">
    <section className="modal command-modal quick-settings-modal" role="dialog" aria-modal="true" aria-label={t("gifts.quickSettings.title", { id: gift.GiftID })}>
      <div className="modal-head">
        <div><div className="eyebrow">{t("gifts.quickSettings.eyebrow")}</div><h2>{t("gifts.quickSettings.title", { id: gift.GiftID })}</h2><p>{gift.Title || `Gift #${gift.GiftID}`}</p></div>
        <button className="icon-btn" type="button" onClick={onClose} aria-label={t("action.close")}><X size={15} /></button>
      </div>
      <div className="command-body quick-settings-body">
        <section className="quick-settings-section">
          <div className="quick-settings-head"><strong>{t("gifts.sortOrder")}</strong><span>{t("gifts.quickSettings.sortOrderHint")}</span></div>
          <div className="quick-settings-row">
            <label><input type="number" value={sortOrder} onChange={(e) => setSortOrder(e.target.value)} /></label>
            <ActionButton compact tone="neutral" label={t("gifts.quickSettings.saveOrder")}
              path="/api/actions/set-gift-sort-order"
              payload={() => ({ gift_id: gift.GiftID, sort_order: Number(sortOrder) })}
              onDone={onChanged} />
          </div>
        </section>

        <section className="quick-settings-section">
          <div className="quick-settings-head"><strong>{t("gifts.quickSettings.supply")}</strong><span>{t("gifts.quickSettings.supplyHint")}</span></div>
          <label className="gift-switch"><input type="checkbox" checked={limited} onChange={(e) => setLimited(e.target.checked)} /><span className="gift-switch-track" aria-hidden="true"><span /></span><span>{t("gifts.limited.label")}</span></label>
          {limited && <div className="quick-settings-row">
            <label><span>{t("gifts.limited.total")}</span><input type="number" min="1" value={total} onChange={(e) => setTotal(e.target.value)} /></label>
            <label><span>{t("gifts.limited.issued")}</span><input type="number" min="0" value={issued} onChange={(e) => setIssued(e.target.value)} /></label>
          </div>}
          <ActionButton compact tone="neutral" label={t("gifts.quickSettings.saveSupply")}
            path="/api/actions/set-gift-supply"
            payload={() => ({ gift_id: gift.GiftID, limited, availability_total: limited ? Number(total) : 0, availability_issued: limited ? Number(issued) : 0 })}
            onDone={onChanged} />
        </section>

        <section className="quick-settings-section quick-settings-danger">
          <div className="quick-settings-head"><strong>{t("gifts.quickSettings.deleteTitle")}</strong><span>{t("gifts.quickSettings.deleteHint")}</span></div>
          <label className="gift-switch"><input type="checkbox" checked={refundStars} onChange={(e) => setRefundStars(e.target.checked)} /><span className="gift-switch-track" aria-hidden="true"><span /></span><span>{t("gifts.quickSettings.refundStars")}</span></label>
          <ActionButton tone="danger" icon={<Trash2 size={15} />} label={t("gifts.quickSettings.deleteButton")}
            path="/api/actions/delete-gift"
            payload={() => ({ gift_id: gift.GiftID, refund_stars: refundStars })}
            onDone={() => { onChanged(); onClose(); }} />
        </section>
      </div>
    </section>
  </div>, document.body);
}
