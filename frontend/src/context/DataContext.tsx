import React, { createContext, useContext, useState, useEffect } from 'react';
import type { AppData } from '../types';
import { buildEmpty, loadAll, enrich } from '../api';

interface DataContextValue {
  data: AppData;
  status: 'loading' | 'live' | 'demo';
  refresh: () => void;
}

const DataContext = createContext<DataContextValue | null>(null);

export function DataProvider({ children }: { children: React.ReactNode }) {
  const [data, setData] = useState<AppData>(buildEmpty);
  const [status, setStatus] = useState<'loading' | 'live' | 'demo'>('loading');
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let alive = true;
    setStatus('loading');
    loadAll()
      .then(({ data: d }) => {
        if (!alive) return;
        setData(d);
        setStatus(d.__live ? 'live' : 'demo');
        enrich(d, () => { if (alive) setData({ ...d }); });
      })
      .catch(() => { if (alive) setStatus('demo'); });
    return () => { alive = false; };
  }, [tick]);

  return (
    <DataContext.Provider value={{ data, status, refresh: () => setTick((t) => t + 1) }}>
      {children}
    </DataContext.Provider>
  );
}

export function useData(): DataContextValue {
  const ctx = useContext(DataContext);
  if (!ctx) throw new Error('useData must be used within DataProvider');
  return ctx;
}
