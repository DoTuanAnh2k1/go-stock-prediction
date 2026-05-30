export interface FmtUtils {
  vnd(n: number): string;
  price(n: number): string;
  pct(n: number): string;
  sign(n: number): string;
  compact(n: number): string;
  goldShort(n: number): string;
}

export interface IndexData {
  val: number;
  chg: number;
  chgPct: number;
  vol: number;
  series: number[];
}

export interface StockItem {
  sym: string;
  name: string;
  sector: string;
  exchange: string;
  price: number;
  change: number;
  chgPct: number;
  volume: number;
  value: number;
  vn30: boolean;
  spark: number[];
  hist: number[];
}

export interface MoverItem {
  sym: string;
  name: string;
  price: number;
  chgPct: number;
  volume: number;
  spark: number[];
}

export interface PredictionItem {
  sym: string;
  name: string;
  algo: string;
  algoShort: string;
  algoCls: string;
  cur: number;
  pred: number;
  deltaPct: number;
  conf: number;
  date: string;
  target: string;
}

export interface ConfirmedItem {
  sym: string;
  name: string;
  algo: string;
  algoShort: string;
  algoCls: string;
  pred: number;
  actual: number;
  cur: number;
  predDelta: number;
  actualDelta: number;
  conf: number;
  acc: number;
  err: number;
  target: string;
  status: 'hit' | 'close' | 'miss';
}

export interface AlgoItem {
  id: string;
  short: string;
  name: string;
  cls: string;
  desc: string;
  acc: number;
  accDelta: number;
  mae: number;
  trainedAt: string;
  epochs: number | null;
  status: string;
}

export interface GoldSource {
  id: string;
  name: string;
  vendor: string;
  region: string;
  unit: string;
  source: string;
  product: string;
  buy: number;
  sell: number;
  chg: number;
  chgPct: number;
  spark: number[];
  hist: number[];
}

export interface GoldPred {
  src: string;
  type: string;
  algo: string;
  algoShort: string;
  algoCls: string;
  cur: number;
  pred: number;
  deltaPct: number;
  conf: number;
  date: string;
  isOz: boolean;
}

export interface GoldPredActual {
  labels: string[];
  actual: (number | null)[];
  pred: (number | null)[];
}

export interface GoldDetailItem {
  date: string;
  buy: number;
  sell: number;
  spread: number;
}

export interface TrainJob {
  name: string;
  sub: string;
  status: 'running' | 'done' | 'pending';
  progress: number;
  eta: string;
}

export interface AppData {
  fmt: FmtUtils;
  stocks: StockItem[];
  predictions: PredictionItem[];
  confirmed: ConfirmedItem[];
  algos: AlgoItem[];
  gainers: MoverItem[];
  losers: MoverItem[];
  active: MoverItem[];
  goldSources: GoldSource[];
  goldPreds: GoldPred[];
  goldPredActual: GoldPredActual | null;
  goldDetail: GoldDetailItem[];
  stats: { total: number; acc: number };
  indices: { vnindex: IndexData; vn30: IndexData };
  accTrend: { labels: string[]; lstm: (number | null)[]; arima: (number | null)[]; ma: (number | null)[] };
  dailyCounts: { labels: string[]; values: number[] };
  trainLogs: [string, string, string][];
  trainJobs: TrainJob[];
  __live: boolean;
  __sources: Record<string, boolean>;
}
