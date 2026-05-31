import { Icon, Panel } from '../components/ui';

const STEPS: { n: number; code: string; desc: string }[] = [
  {
    n: 1,
    code: 'pkg/service/market/crypto/market.go',
    desc: 'Implement the AssetMarket interface — price fetching, normalisation, exchange mapping.',
  },
  {
    n: 2,
    code: 'pkg/models/models_db/crypto_price.go\npkg/models/models_db/crypto_prediction.go',
    desc: 'Add GORM structs and register them in migrations.go.',
  },
  {
    n: 3,
    code: 'pkg/service/crawler/crypto_crawler.go',
    desc: 'Add crawler (e.g. Binance public API) and register the cron job in crawler/init.go.',
  },
  {
    n: 4,
    code: 'proto/prediction/prediction.proto\npkg/server/api_trigger_crypto.go\npkg/server/router.go',
    desc: 'Add gRPC RPC definition, regenerate .pb.go, add HTTP trigger handler and register routes.',
  },
  {
    n: 5,
    code: 'frontend/src/pages/Crypto.tsx',
    desc: 'Remove this placeholder and build the full page with KPIs, charts, and predictions — following the Gold.tsx pattern.',
  },
];

export default function Crypto() {
  return (
    <div className="content__inner fade">

      {/* ── Coming-soon banner ─────────────────────────────────────────── */}
      <div
        className="panel section-gap"
        style={{
          borderColor: 'var(--accent)',
          background: 'color-mix(in oklch, var(--accent) 6%, var(--bg-1))',
        }}
      >
        <div className="panel__body" style={{ display: 'flex', alignItems: 'flex-start', gap: 20 }}>
          <div
            style={{
              flexShrink: 0,
              width: 48,
              height: 48,
              borderRadius: 12,
              background: 'color-mix(in oklch, var(--accent) 15%, transparent)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'var(--accent)',
            }}
          >
            <Icon name="crypto" size={24} />
          </div>
          <div>
            <div style={{ fontWeight: 600, fontSize: 17, marginBottom: 6 }}>
              Cryptocurrency Predictions
            </div>
            <div style={{ color: 'var(--text-2)', fontSize: 13.5, lineHeight: 1.65, maxWidth: 560 }}>
              This market module is not yet active. The architecture already supports
              multiple asset markets — adding crypto follows the same pattern as the
              Gold module. Complete the five steps below and the page will be fully
              functional.
            </div>
          </div>
          <span
            className="badge badge--muted"
            style={{ flexShrink: 0, marginLeft: 'auto', alignSelf: 'flex-start' }}
          >
            Coming Soon
          </span>
        </div>
      </div>

      {/* ── Activation checklist ───────────────────────────────────────── */}
      <div className="sec-head section-gap">
        <h2>Activation Checklist</h2>
        <div className="line"></div>
      </div>

      <Panel title="5 steps to activate the Crypto market" className="section-gap">
        <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 0 }}>
          {STEPS.map((s, idx) => (
            <li
              key={s.n}
              style={{
                display: 'flex',
                gap: 16,
                padding: '18px 0',
                borderBottom: idx < STEPS.length - 1 ? '1px solid var(--border)' : 'none',
              }}
            >
              {/* Step number bubble */}
              <div
                style={{
                  flexShrink: 0,
                  width: 28,
                  height: 28,
                  borderRadius: '50%',
                  border: '1.5px solid var(--border-strong)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 12,
                  fontWeight: 700,
                  fontFamily: 'var(--font-mono)',
                  color: 'var(--text-2)',
                  marginTop: 1,
                }}
              >
                {s.n}
              </div>

              <div style={{ flex: 1, minWidth: 0 }}>
                {/* File path(s) */}
                <div
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: 12,
                    color: 'var(--accent)',
                    background: 'color-mix(in oklch, var(--accent) 8%, var(--bg-2))',
                    border: '1px solid color-mix(in oklch, var(--accent) 20%, var(--border))',
                    borderRadius: 6,
                    padding: '6px 10px',
                    marginBottom: 8,
                    whiteSpace: 'pre',
                    overflowX: 'auto',
                    lineHeight: 1.7,
                  }}
                >
                  {s.code}
                </div>
                {/* Description */}
                <div style={{ fontSize: 13, color: 'var(--text-2)', lineHeight: 1.6 }}>
                  {s.desc}
                </div>
              </div>
            </li>
          ))}
        </ol>
      </Panel>

      {/* ── Info footer ────────────────────────────────────────────────── */}
      <Panel className="section-gap" style={{ borderStyle: 'dashed' }}>
        <div className="panel__body" style={{ display: 'flex', gap: 32, flexWrap: 'wrap' }}>
          {[
            { label: 'Suggested sources', value: 'Binance, CoinGecko, CryptoCompare' },
            { label: 'Algorithms', value: 'Reuse MA / LSTM / ARIMA-GARCH / Ensemble' },
            { label: 'DB tables', value: 'crypto_prices, crypto_predictions' },
            { label: 'Cron schedule', value: 'Daily12PM crawl · Daily6PM predict' },
          ].map((item) => (
            <div key={item.label} style={{ minWidth: 180 }}>
              <div style={{ fontSize: 11, color: 'var(--text-3)', textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 4 }}>
                {item.label}
              </div>
              <div style={{ fontSize: 13, color: 'var(--text-2)', fontFamily: 'var(--font-mono)' }}>
                {item.value}
              </div>
            </div>
          ))}
        </div>
      </Panel>

    </div>
  );
}
