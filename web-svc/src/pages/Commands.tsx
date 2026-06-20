import { useState, useEffect, useCallback } from 'react';
import { Panel } from '../components/ui';
import { useLanguage } from '../context/LangContext';
import './MarketGroups.css';

// ── Types ─────────────────────────────────────────────────────────────────────

interface ArgSchema {
  name: string;
  required: boolean;
  type: 'string' | 'number' | 'boolean';
  choices?: string[];
}

interface CliHandler {
  handler_key: string;
  display_name: string;
  verb: string;
  resource: string;
  arg_schema: ArgSchema[];
  enabled: boolean;
}

interface Command {
  id: number;
  name: string;
  description: string;
  handler_key: string;
  args: Record<string, string>;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

// ── Auth helpers ──────────────────────────────────────────────────────────────

function getToken() { return localStorage.getItem('vns_token') ?? ''; }
function authHeaders(): Record<string, string> {
  return { Authorization: `Bearer ${getToken()}`, 'Content-Type': 'application/json' };
}

function unwrap(data: unknown): unknown {
  if (data && typeof data === 'object' && 'data' in data) {
    return (data as { data: unknown }).data;
  }
  return data;
}

// ── Req asterisk ──────────────────────────────────────────────────────────────

function Req() {
  return <span style={{ color: 'var(--down)' }}> *</span>;
}

// ── Verb badge ────────────────────────────────────────────────────────────────

const VERB_COLOR: Record<string, string> = {
  get:    'oklch(0.72 0.15 200)',
  set:    'oklch(0.72 0.16 152)',
  update: 'oklch(0.74 0.13 60)',
  delete: 'var(--down)',
};

function VerbBadge({ verb }: { verb: string }) {
  const color = VERB_COLOR[verb] ?? 'var(--text-2)';
  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center',
      padding: '1px 7px', borderRadius: 4,
      fontSize: 11, fontWeight: 700, fontFamily: 'var(--font-mono)',
      background: `color-mix(in oklch, ${color} 15%, transparent)`,
      color, border: `1px solid color-mix(in oklch, ${color} 35%, transparent)`,
    }}>
      {verb.toUpperCase()}
    </span>
  );
}

// ── Dynamic args form ─────────────────────────────────────────────────────────

interface ArgsFormProps {
  schema: ArgSchema[];
  values: Record<string, string>;
  onChange: (key: string, val: string) => void;
}

function ArgsForm({ schema, values, onChange }: ArgsFormProps) {
  const { t } = useLanguage();
  if (schema.length === 0) {
    return <div style={{ fontSize: 12, color: 'var(--text-3)', fontStyle: 'italic' }}>{t.commands.noArgs}</div>;
  }
  return (
    <div style={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
      gap: '8px 14px',
    }}>
      {schema.map(arg => (
        <div key={arg.name} className="field" style={{ minWidth: 0, margin: 0 }}>
          <label className="field__label" style={{ fontSize: 12 }}>
            {arg.name}{arg.required && <Req />}
            <span style={{ color: 'var(--text-3)', fontWeight: 400, marginLeft: 4, fontSize: 11 }}>
              ({arg.type})
            </span>
          </label>
          {arg.choices && arg.choices.length > 0 ? (
            <select
              className="field__input"
              value={values[arg.name] ?? ''}
              onChange={e => onChange(arg.name, e.target.value)}
              style={{ fontSize: 13 }}
            >
              <option value="">-- {arg.required ? t.commands.selectRequired : t.commands.selectOptional} --</option>
              {arg.choices.map(c => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
          ) : (
            <input
              className="field__input"
              type={arg.type === 'number' ? 'number' : 'text'}
              value={values[arg.name] ?? ''}
              onChange={e => onChange(arg.name, e.target.value)}
              placeholder={arg.name}
              style={{ fontSize: 13 }}
            />
          )}
        </div>
      ))}
    </div>
  );
}

// ── Command modal (create / edit) ─────────────────────────────────────────────

interface CommandModalProps {
  handlers: CliHandler[];
  initial?: Command;
  onClose: () => void;
  onSaved: () => void;
  strings: ReturnType<typeof useLanguage>['t']['commands'];
}

function CommandModal({ handlers, initial, onClose, onSaved, strings }: CommandModalProps) {
  const [name, setName] = useState(initial?.name ?? '');
  const [description, setDescription] = useState(initial?.description ?? '');
  const [handlerKey, setHandlerKey] = useState(initial?.handler_key ?? '');
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [argValues, setArgValues] = useState<Record<string, string>>(
    initial?.args ? Object.fromEntries(Object.entries(initial.args).map(([k, v]) => [k, String(v)])) : {}
  );
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const selectedHandler = handlers.find(h => h.handler_key === handlerKey);

  const handleHandlerChange = (key: string) => {
    setHandlerKey(key);
    setArgValues({});
  };

  const handleArgChange = (key: string, val: string) => {
    setArgValues(prev => ({ ...prev, [key]: val }));
  };

  const validate = () => {
    if (!name.trim()) return strings.nameRequired;
    if (!handlerKey) return strings.handlerRequired;
    if (selectedHandler) {
      for (const arg of selectedHandler.arg_schema) {
        if (arg.required && !argValues[arg.name]?.trim()) {
          return `${strings.argRequired}: ${arg.name}`;
        }
      }
    }
    return '';
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const err = validate();
    if (err) { setError(err); return; }
    setError('');
    setSubmitting(true);

    // Build clean args object (only non-empty values)
    const cleanArgs: Record<string, string> = {};
    for (const [k, v] of Object.entries(argValues)) {
      if (v.trim()) cleanArgs[k] = v.trim();
    }

    const body = {
      name: name.trim(),
      description: description.trim(),
      handler_key: handlerKey,
      args: cleanArgs,
      enabled,
    };

    try {
      const url = initial ? `/api/commands/${initial.id}` : '/api/commands';
      const method = initial ? 'PUT' : 'POST';
      const res = await fetch(url, { method, headers: authHeaders(), body: JSON.stringify(body) });
      const data = await res.json().catch(() => ({})) as { message?: string };
      if (!res.ok) throw new Error(data.message || strings.saveFail);
      onSaved();
      onClose();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : strings.saveFail);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={e => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="modal-box" style={{ maxWidth: 580, width: '100%' }}>
        <div className="modal-header">
          <span className="modal-title">{initial ? strings.editCommand : strings.createCommand}</span>
          <button
            className="btn btn--icon btn--ghost"
            onClick={onClose}
            style={{ marginLeft: 'auto' }}
            aria-label="Close"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>
        <form className="modal-body" onSubmit={handleSubmit}>
          {error && <div className="modal-error">{error}</div>}

          <div style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
            gap: '10px 14px',
            marginBottom: 12,
          }}>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">{strings.commandName}<Req /></label>
              <input
                className="field__input"
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder={strings.commandName}
                autoFocus
              />
            </div>
            <div className="field" style={{ minWidth: 0 }}>
              <label className="field__label">
                {strings.description}
                <span style={{ color: 'var(--text-3)', fontWeight: 400 }}> ({strings.optional})</span>
              </label>
              <input
                className="field__input"
                value={description}
                onChange={e => setDescription(e.target.value)}
                placeholder={strings.descriptionPlaceholder}
              />
            </div>
          </div>

          <div className="field" style={{ marginBottom: 12 }}>
            <label className="field__label">{strings.handler}<Req /></label>
            <select
              className="field__input"
              value={handlerKey}
              onChange={e => handleHandlerChange(e.target.value)}
            >
              <option value="">-- {strings.selectHandler} --</option>
              {handlers.filter(h => h.enabled).map(h => (
                <option key={h.handler_key} value={h.handler_key}>
                  [{h.verb.toUpperCase()}] {h.display_name} ({h.resource})
                </option>
              ))}
            </select>
          </div>

          {selectedHandler && (
            <div style={{ marginBottom: 12 }}>
              <div style={{ fontSize: 11, fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em', color: 'var(--text-2)', marginBottom: 8 }}>
                {strings.argsSection}
              </div>
              <ArgsForm
                schema={selectedHandler.arg_schema}
                values={argValues}
                onChange={handleArgChange}
              />
            </div>
          )}

          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
            <input
              id="cmd-enabled"
              type="checkbox"
              checked={enabled}
              onChange={e => setEnabled(e.target.checked)}
              style={{ accentColor: 'var(--accent)', width: 14, height: 14, cursor: 'pointer' }}
            />
            <label htmlFor="cmd-enabled" style={{ fontSize: 13, cursor: 'pointer' }}>{strings.enabled}</label>
          </div>

          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
            <button type="button" className="btn" onClick={onClose} disabled={submitting}>
              {strings.cancel}
            </button>
            <button type="submit" className="btn btn--primary" disabled={submitting || !name.trim() || !handlerKey}>
              {submitting ? strings.saving : strings.save}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Args summary helper ───────────────────────────────────────────────────────

function ArgsSummary({ args }: { args: Record<string, string> }) {
  const entries = Object.entries(args);
  if (entries.length === 0) return <span style={{ color: 'var(--text-3)', fontStyle: 'italic', fontSize: 12 }}>—</span>;
  return (
    <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>
      {entries.map(([k, v]) => `${k}=${v}`).join(', ')}
    </span>
  );
}

// ── Main Commands page ────────────────────────────────────────────────────────

export default function Commands() {
  const { t } = useLanguage();
  const s = t.commands;

  const [commands, setCommands] = useState<Command[]>([]);
  const [handlers, setHandlers] = useState<CliHandler[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showModal, setShowModal] = useState(false);
  const [editTarget, setEditTarget] = useState<Command | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const [cmdRes, hdlRes] = await Promise.all([
        fetch('/api/commands', { headers: authHeaders() }),
        fetch('/api/command-handlers', { headers: authHeaders() }),
      ]);
      if (!cmdRes.ok) throw new Error(s.cannotLoad);
      const cmdRaw = await cmdRes.json();
      setCommands((unwrap(cmdRaw) as Command[]) ?? []);

      if (hdlRes.ok) {
        const hdlRaw = await hdlRes.json();
        setHandlers((unwrap(hdlRaw) as CliHandler[]) ?? []);
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : s.cannotLoad);
    } finally {
      setLoading(false);
    }
  }, [s.cannotLoad]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleDelete = async (cmd: Command) => {
    if (!confirm(`${s.deleteConfirm} "${cmd.name}"?`)) return;
    const res = await fetch(`/api/commands/${cmd.id}`, { method: 'DELETE', headers: authHeaders() });
    if (res.ok) fetchData();
    else setError(s.deleteFail);
  };

  const handlerDisplayName = (key: string) => {
    const h = handlers.find(x => x.handler_key === key);
    return h ? h.display_name : key;
  };

  const handlerVerb = (key: string) => {
    const h = handlers.find(x => x.handler_key === key);
    return h?.verb ?? '';
  };

  return (
    <div className="mg-page">
      {(showModal || editTarget) && (
        <CommandModal
          handlers={handlers}
          initial={editTarget ?? undefined}
          onClose={() => { setShowModal(false); setEditTarget(null); }}
          onSaved={fetchData}
          strings={s}
        />
      )}

      {error && (
        <div className="mg-error-banner">
          <span>{error}</span>
          <button className="btn btn--icon btn--ghost" onClick={() => setError('')} aria-label="Dismiss">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
            </svg>
          </button>
        </div>
      )}

      <Panel
        title={s.pageTitle}
        sub={loading ? undefined : `${commands.length} ${s.commandCount}`}
        tools={
          <button className="btn btn--primary" style={{ fontSize: 12 }} onClick={() => { setEditTarget(null); setShowModal(true); }}>
            + {s.createCommand}
          </button>
        }
      >
        {loading ? (
          <div style={{ padding: 16, color: 'var(--text-2)' }}>{s.loading}</div>
        ) : commands.length === 0 ? (
          <div style={{ padding: '16px 0', color: 'var(--text-3)', fontStyle: 'italic', fontSize: 13 }}>
            {s.noCommands}
          </div>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: '1px solid var(--border)' }}>
                  <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{s.colName}</th>
                  <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{s.colHandler}</th>
                  <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{s.colArgs}</th>
                  <th style={{ textAlign: 'left', padding: '8px 12px', color: 'var(--text-2)', fontWeight: 500 }}>{s.colStatus}</th>
                  <th style={{ width: 80 }}></th>
                </tr>
              </thead>
              <tbody>
                {commands.map(cmd => (
                  <tr key={cmd.id} style={{ borderBottom: '1px solid var(--border)' }}>
                    <td style={{ padding: '8px 12px' }}>
                      <div style={{ fontWeight: 600 }}>{cmd.name}</div>
                      {cmd.description && (
                        <div style={{ fontSize: 12, color: 'var(--text-3)', marginTop: 2 }}>{cmd.description}</div>
                      )}
                    </td>
                    <td style={{ padding: '8px 12px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        {handlerVerb(cmd.handler_key) && (
                          <VerbBadge verb={handlerVerb(cmd.handler_key)} />
                        )}
                        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>
                          {handlerDisplayName(cmd.handler_key)}
                        </span>
                      </div>
                    </td>
                    <td style={{ padding: '8px 12px' }}>
                      <ArgsSummary args={cmd.args} />
                    </td>
                    <td style={{ padding: '8px 12px' }}>
                      <span className={`badge badge--${cmd.enabled ? 'up' : 'muted'}`}>
                        {cmd.enabled ? s.enabled : s.disabled}
                      </span>
                    </td>
                    <td style={{ padding: '8px 12px', display: 'flex', gap: 4, alignItems: 'center' }}>
                      <button
                        className="btn btn--icon btn--ghost"
                        style={{ color: 'var(--text-2)' }}
                        onClick={() => { setEditTarget(cmd); setShowModal(false); }}
                        title={s.edit}
                      >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
                          <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                          <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                        </svg>
                      </button>
                      <button
                        className="btn btn--icon btn--ghost"
                        style={{ color: 'var(--down)' }}
                        onClick={() => handleDelete(cmd)}
                        title={s.delete}
                      >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
                          <polyline points="3 6 5 6 21 6"/>
                          <path d="M19 6l-1 14H6L5 6"/>
                          <path d="M10 11v6"/><path d="M14 11v6"/><path d="M9 6V4h6v2"/>
                        </svg>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>
    </div>
  );
}
