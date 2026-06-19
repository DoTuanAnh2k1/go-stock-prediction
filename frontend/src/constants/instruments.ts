// Per-market instrument lists shared by MarketDetail (algorithm comparison) and
// MarketPredictions (per-symbol prediction chart). Only includes instruments that
// are actually crawled & predicted — e.g. QQQ/SPY ETFs are NOT crawled and would
// render an empty chart, so they are intentionally excluded.

export interface Instrument {
  key: string;
  label: string;
}

export const GOLD_INSTRUMENTS: Instrument[] = [
  { key: 'XAU', label: 'XAU/USD (Spot)' },
  { key: 'BTMC_SJC', label: 'BTMC/SJC' },
  { key: 'BTMC_NHAN', label: 'BTMC/Nhẫn tròn' },
];

export const NASDAQ_INSTRUMENTS: Instrument[] = [
  { key: 'AAPL', label: 'AAPL' },
  { key: 'MSFT', label: 'MSFT' },
  { key: 'NVDA', label: 'NVDA' },
  { key: 'GOOGL', label: 'GOOGL' },
  { key: 'AMZN', label: 'AMZN' },
  { key: 'META', label: 'META' },
  { key: 'TSLA', label: 'TSLA' },
  { key: 'AMD', label: 'AMD' },
  { key: 'AVGO', label: 'AVGO' },
  { key: 'NFLX', label: 'NFLX' },
  { key: 'COST', label: 'COST' },
  { key: 'ADBE', label: 'ADBE' },
  { key: 'CSCO', label: 'CSCO' },
  { key: 'QCOM', label: 'QCOM' },
  { key: 'INTC', label: 'INTC' },
];

export const CRYPTO_INSTRUMENTS: Instrument[] = [
  { key: 'bitcoin', label: 'Bitcoin (BTC)' },
  { key: 'ethereum', label: 'Ethereum (ETH)' },
  { key: 'solana', label: 'Solana (SOL)' },
];

// instrumentsFor returns the instrument list for a market route key.
export function instrumentsFor(marketKey: string): Instrument[] {
  switch (marketKey) {
    case 'nasdaq100': return NASDAQ_INSTRUMENTS;
    case 'crypto':    return CRYPTO_INSTRUMENTS;
    case 'gold':      return GOLD_INSTRUMENTS;
    default:          return GOLD_INSTRUMENTS;
  }
}

// defaultInstrument returns the default selected instrument key for a market.
export function defaultInstrument(marketKey: string): string {
  return instrumentsFor(marketKey)[0]?.key ?? '';
}
