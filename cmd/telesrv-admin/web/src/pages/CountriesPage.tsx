import { Globe2, Loader2, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { api, errorMessage } from "../api";
import { ActionButton } from "../components/ActionButton";
import { Alert, Badge, EmptyRow, PageFrame, SectionHead } from "../components/ui";
import type { Country } from "../types";

function splitList(value: string): string[] {
  return value
    .split(",")
    .map((v) => v.trim())
    .filter((v) => v.length > 0);
}

export function CountriesPage() {
  const [countries, setCountries] = useState<Country[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [iso2, setIso2] = useState("");
  const [defaultName, setDefaultName] = useState("");
  const [name, setName] = useState("");
  const [hidden, setHidden] = useState(false);
  const [code, setCode] = useState("");
  const [prefixes, setPrefixes] = useState("");
  const [patterns, setPatterns] = useState("");

  async function load() {
    setBusy(true);
    setError("");
    try {
      const result = await api.countries();
      setCountries(result.countries ?? []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const canAdd = iso2.trim().length >= 2 && defaultName.trim().length > 0 && code.trim().length > 0;

  return (
    <PageFrame
      title={"Коды стран"}
      eyebrow={"help.getCountriesList / экран входа"}
      actions={
        <button className="btn icon-text" type="button" onClick={() => void load()} disabled={busy}>
          {busy ? <Loader2 size={15} className="spin" /> : <RefreshCw size={15} />} {"Обновить"}
        </button>
      }
    >
      {error && <Alert>{error}</Alert>}
      <p className="field-hint">
        {
          "Список, который клиент подгружает на экране входа (телефон/страна) и в выборе страны. ISO2, который не совпадает ни с одним настоящим кодом из встроенного каталога (~235 стран) — это новая, полностью своя запись, она переживает перезапуск сервера как есть. Если указать ISO2 существующей настоящей страны, правки применятся, но каталог при следующем перезапуске молча вернёт её к официальным значениям."
        }
      </p>

      <section className="section-block">
        <SectionHead title={"Добавить свой код"} text={"Например: ISO2=SG, код 999 — свой диапазон авторизации, не пересекающийся с реальными странами."} />
        <div className="card-body">
          <div className="attr-block">
            <label className="duration-field">
              <span>{"ISO2"}</span>
              <input value={iso2} onChange={(event) => setIso2(event.target.value.toUpperCase())} maxLength={4} placeholder="напр. SG" />
            </label>
            <label className="duration-field">
              <span>{"Название"}</span>
              <input value={defaultName} onChange={(event) => setDefaultName(event.target.value)} placeholder="напр. ShuzaGram Anonymous" />
            </label>
            <label className="duration-field">
              <span>{"Название (override), опционально"}</span>
              <input value={name} onChange={(event) => setName(event.target.value)} placeholder="если пусто — берётся Название" />
            </label>
            <label className="duration-field">
              <span>{"Код (цифры)"}</span>
              <input value={code} onChange={(event) => setCode(event.target.value.replace(/[^0-9]/g, ""))} placeholder="напр. 999" />
            </label>
            <label className="duration-field">
              <span>{"Префиксы, через запятую"}</span>
              <input value={prefixes} onChange={(event) => setPrefixes(event.target.value)} placeholder="можно оставить пустым" />
            </label>
            <label className="duration-field">
              <span>{"Формат номера, через запятую"}</span>
              <input value={patterns} onChange={(event) => setPatterns(event.target.value)} placeholder="напр. XXX XXXX XXXX" />
            </label>
            <label className="checkbox-field">
              <input type="checkbox" checked={hidden} onChange={(event) => setHidden(event.target.checked)} />
              <span>{"Скрыт (hidden)"}</span>
            </label>
            <ActionButton
              label={"Добавить / сохранить"}
              icon={<Plus size={15} />}
              tone="neutral"
              path="/api/actions/upsert-country"
              disabled={!canAdd}
              payload={() => ({
                iso2: iso2.trim(),
                default_name: defaultName.trim(),
                name: name.trim(),
                hidden,
                codes: [
                  {
                    country_code: code.trim(),
                    prefixes: splitList(prefixes),
                    patterns: splitList(patterns)
                  }
                ]
              })}
              onDone={() => {
                setIso2("");
                setDefaultName("");
                setName("");
                setHidden(false);
                setCode("");
                setPrefixes("");
                setPatterns("");
                void load();
              }}
            />
          </div>
        </div>
      </section>

      <section className="section-block">
        <SectionHead title={`Все записи (${countries.length})`} text={"Полный каталог, включая встроенные ~235 стран и любые свои записи."} />
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>{"ISO2"}</th>
                <th>{"Название"}</th>
                <th>{"Коды"}</th>
                <th>{"Флаги"}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {countries.map((c) => (
                <tr key={c.iso2}>
                  <td className="mono"><Globe2 size={13} /> {c.iso2}</td>
                  <td>{c.name || c.default_name}</td>
                  <td className="mono">
                    {(c.country_codes ?? []).map((cc) => `+${cc.country_code}`).join(", ")}
                  </td>
                  <td>{c.hidden && <Badge tone="neutral">{"скрыт"}</Badge>}</td>
                  <td>
                    <div className="toolbar">
                      <ActionButton
                        compact
                        label={"Удалить"}
                        icon={<Trash2 size={13} />}
                        tone="danger"
                        path="/api/actions/delete-country"
                        payload={() => ({ iso2: c.iso2 })}
                        onDone={load}
                      />
                    </div>
                  </td>
                </tr>
              ))}
              {countries.length === 0 && <EmptyRow colSpan={5} />}
            </tbody>
          </table>
        </div>
      </section>
    </PageFrame>
  );
}
