import { createContext, useContext, useState } from 'react';
import type { ReactNode } from 'react';
import { translations } from '../i18n';
import type { Lang, Translations } from '../i18n';

interface LangContextType {
  lang: Lang;
  toggleLang: () => void;
  t: Translations;
}

const LangContext = createContext<LangContextType | null>(null);

export function LangProvider({ children }: { children: ReactNode }) {
  const [lang, setLang] = useState<Lang>(
    () => (localStorage.getItem('vns_lang') as Lang) || 'vi'
  );

  function toggleLang() {
    const next: Lang = lang === 'vi' ? 'en' : 'vi';
    setLang(next);
    localStorage.setItem('vns_lang', next);
  }

  return (
    <LangContext.Provider value={{ lang, toggleLang, t: translations[lang] }}>
      {children}
    </LangContext.Provider>
  );
}

export function useLanguage() {
  const ctx = useContext(LangContext);
  if (!ctx) throw new Error('useLanguage must be used within LangProvider');
  return ctx;
}
