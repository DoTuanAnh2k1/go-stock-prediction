import {
  useState,
  useEffect,
  useRef,
  useCallback,
  useMemo,
} from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import 'highlight.js/styles/github-dark.css';
import './Docs.css';
import './docs-viz/docs-viz.css';
import { useAuth } from '../context/AuthContext';
import { Icon } from '../components/ui';
import MermaidBlock from './docs-viz/MermaidBlock';
import VizBlock from './docs-viz/VizBlock';

// ── Types ─────────────────────────────────────────────────────────────────────

interface DocMeta {
  path: string;
  title: string;
  group: string;
  order: number;
}

interface DocContent {
  path: string;
  title: string;
  content: string;
}

interface TocItem {
  id: string;
  text: string;
  level: 2 | 3;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function getToken(): string {
  return localStorage.getItem('vns_token') || '';
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\w\s-]/g, '')
    .trim()
    .replace(/[\s_]+/g, '-')
    .replace(/-+/g, '-');
}

function readingTime(content: string): number {
  const words = content.trim().split(/\s+/).length;
  return Math.max(1, Math.ceil(words / 200));
}

function extractToc(content: string): TocItem[] {
  const lines = content.split('\n');
  const items: TocItem[] = [];
  const seenIds = new Map<string, number>();

  for (const line of lines) {
    const m2 = line.match(/^##\s+(.+)/);
    const m3 = line.match(/^###\s+(.+)/);
    const raw = m2?.[1] ?? m3?.[1];
    if (!raw) continue;
    const level = m2 ? 2 : 3;
    const base = slugify(raw);
    const count = seenIds.get(base) ?? 0;
    seenIds.set(base, count + 1);
    const id = count === 0 ? base : `${base}-${count}`;
    items.push({ id, text: raw, level: level as 2 | 3 });
  }

  return items;
}

function groupDocs(docs: DocMeta[]): [string, DocMeta[]][] {
  const map = new Map<string, DocMeta[]>();
  for (const d of docs) {
    if (!map.has(d.group)) map.set(d.group, []);
    map.get(d.group)!.push(d);
  }
  // Sort within groups by order
  for (const arr of map.values()) {
    arr.sort((a, b) => a.order - b.order);
  }
  return Array.from(map.entries());
}

// ── Custom heading component for ReactMarkdown ────────────────────────────────

function makeHeading(level: 2 | 3 | 4) {
  const Tag = `h${level}` as 'h2' | 'h3' | 'h4';
  const seenIds = new Map<string, number>();

  return function Heading({
    children,
    ...props
  }: React.HTMLAttributes<HTMLHeadingElement>) {
    const text = typeof children === 'string'
      ? children
      : Array.isArray(children)
        ? children.map(c => (typeof c === 'string' ? c : '')).join('')
        : '';
    const base = slugify(text);
    const count = seenIds.get(base) ?? 0;
    seenIds.set(base, count + 1);
    const id = count === 0 ? base : `${base}-${count}`;
    return <Tag id={id} {...props}>{children}</Tag>;
  };
}

// We create these once and reuse so the maps don't reset on every render
const H2 = makeHeading(2);
const H3 = makeHeading(3);
const H4 = makeHeading(4);

// Wrap tables so wide ones scroll horizontally instead of crushing columns.
function TableWrap(props: React.HTMLAttributes<HTMLTableElement>) {
  return (
    <div className="docs-table-wrap">
      <table {...props} />
    </div>
  );
}

// Custom <pre> renderer: detect mermaid/viz code blocks and route to custom components.
// react-markdown renders fenced code as <pre><code className="language-xxx">...</code></pre>.
// We intercept at the <pre> level so regular code blocks still get rehypeHighlight styling.
interface PreProps extends React.HTMLAttributes<HTMLPreElement> {
  children?: React.ReactNode;
}

// Recursively flatten React children to raw text — needed because rehypeHighlight
// may tokenize code content into nested <span> elements instead of a plain string.
function extractText(node: React.ReactNode): string {
  if (node == null || node === false) return '';
  if (typeof node === 'string') return node;
  if (typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(extractText).join('');
  if (typeof node === 'object' && 'props' in node) {
    return extractText((node as React.ReactElement<{ children?: React.ReactNode }>).props.children);
  }
  return '';
}

function DocsPre({ children, ...rest }: PreProps) {
  // Children is normally a single <code> element
  const child = Array.isArray(children) ? children[0] : children;
  if (child && typeof child === 'object' && 'props' in child) {
    const codeProps = (child as React.ReactElement<{ className?: string; children?: React.ReactNode }>).props;
    // rehypeHighlight may render className as "hljs language-mermaid" — match loosely.
    const className: string = codeProps.className ?? '';
    const rawContent = extractText(codeProps.children);

    if (className.includes('language-mermaid')) {
      return <MermaidBlock code={rawContent.trim()} />;
    }
    if (className.includes('language-viz')) {
      return <VizBlock spec={rawContent.trim()} />;
    }
  }
  // Fallback: normal pre block (rehypeHighlight already applied hljs classes)
  return <pre {...rest}>{children}</pre>;
}

const MD_COMPONENTS = { h2: H2, h3: H3, h4: H4, table: TableWrap, pre: DocsPre };

// ── TOC Rail ──────────────────────────────────────────────────────────────────

function TocRail({ items, readerRef }: { items: TocItem[]; readerRef: React.RefObject<HTMLDivElement | null> }) {
  const [activeId, setActiveId] = useState<string>('');

  useEffect(() => {
    if (!items.length) return;
    const reader = readerRef.current;
    if (!reader) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter(e => e.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible.length > 0) {
          setActiveId(visible[0].target.id);
        }
      },
      {
        root: reader,
        rootMargin: '-10% 0px -70% 0px',
        threshold: 0,
      }
    );

    for (const item of items) {
      const el = reader.querySelector(`#${CSS.escape(item.id)}`);
      if (el) observer.observe(el);
    }

    return () => observer.disconnect();
  }, [items, readerRef]);

  const handleClick = useCallback(
    (id: string) => {
      const reader = readerRef.current;
      if (!reader) return;
      const el = reader.querySelector(`#${CSS.escape(id)}`);
      if (!el) return;
      const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
      el.scrollIntoView({ behavior: prefersReduced ? 'auto' : 'smooth', block: 'start' });
    },
    [readerRef]
  );

  if (!items.length) return null;

  return (
    <aside className="docs-toc">
      <div className="docs-toc__inner">
        <span className="docs-toc__label">Mục lục</span>
        {items.map((item) => (
          <button
            key={item.id}
            className={`docs-toc__item${item.level === 3 ? ' docs-toc__item--h3' : ''}${activeId === item.id ? ' active' : ''}`}
            onClick={() => handleClick(item.id)}
            title={item.text}
          >
            {item.text}
          </button>
        ))}
      </div>
    </aside>
  );
}

// ── Skeleton loader ────────────────────────────────────────────────────────────

function DocSkeleton() {
  return (
    <div className="docs-skeleton">
      <div className="docs-skel-line" style={{ width: '30%', height: 11 }}></div>
      <div className="docs-skel-line docs-skel-line--title"></div>
      <div className="docs-skel-line" style={{ width: '90%' }}></div>
      <div className="docs-skel-line" style={{ width: '80%' }}></div>
      <div className="docs-skel-line docs-skel-line--short"></div>
      <div className="docs-skel-line" style={{ width: '85%', marginTop: 24 }}></div>
      <div className="docs-skel-line" style={{ width: '70%' }}></div>
      <div className="docs-skel-line" style={{ width: '95%' }}></div>
    </div>
  );
}

// ── Main Docs page ─────────────────────────────────────────────────────────────

export default function Docs() {
  const { role } = useAuth();
  const isAdmin = role === 'admin' || role === 'super_admin';

  const [docs, setDocs] = useState<DocMeta[]>([]);
  const [docsLoading, setDocsLoading] = useState(true);
  const [docsError, setDocsError] = useState('');

  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [docContent, setDocContent] = useState<DocContent | null>(null);
  const [contentLoading, setContentLoading] = useState(false);

  const [search, setSearch] = useState('');
  const readerRef = useRef<HTMLDivElement>(null);

  // ── Fetch doc list ────────────────────────────────────────────────────────

  useEffect(() => {
    if (!isAdmin) return;
    const token = getToken();
    fetch('/api/docs', {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(async (r) => {
        if (r.status === 403) throw new Error('403');
        if (!r.ok) throw new Error(String(r.status));
        return r.json();
      })
      .then((data) => {
        const list: DocMeta[] = data?.docs ?? [];
        setDocs(list);
        // Auto-select first doc
        if (list.length > 0) {
          setSelectedPath(list[0].path);
        }
      })
      .catch((e: Error) => {
        setDocsError(e.message === '403' ? 'forbidden' : 'error');
      })
      .finally(() => setDocsLoading(false));
  }, [isAdmin]);

  // ── Fetch doc content ─────────────────────────────────────────────────────

  useEffect(() => {
    if (!selectedPath || !isAdmin) return;
    setContentLoading(true);
    setDocContent(null);
    const token = getToken();
    fetch(`/api/docs/raw?path=${encodeURIComponent(selectedPath)}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(async (r) => {
        if (!r.ok) throw new Error(String(r.status));
        return r.json();
      })
      .then((data) => {
        setDocContent({
          path: data.path,
          title: data.title,
          content: data.content,
        });
        // Scroll reader back to top when switching docs
        if (readerRef.current) {
          readerRef.current.scrollTop = 0;
        }
      })
      .catch(() => {
        setDocContent(null);
      })
      .finally(() => setContentLoading(false));
  }, [selectedPath, isAdmin]);

  // ── Filtered + grouped docs ───────────────────────────────────────────────

  const filteredDocs = useMemo(() => {
    if (!search.trim()) return docs;
    const q = search.toLowerCase();
    return docs.filter(
      (d) =>
        d.title.toLowerCase().includes(q) ||
        d.path.toLowerCase().includes(q)
    );
  }, [docs, search]);

  const groups = useMemo(() => groupDocs(filteredDocs), [filteredDocs]);

  // ── TOC ───────────────────────────────────────────────────────────────────

  const tocItems = useMemo(
    () => (docContent ? extractToc(docContent.content) : []),
    [docContent]
  );

  const currentMeta = useMemo(
    () => docs.find((d) => d.path === selectedPath) ?? null,
    [docs, selectedPath]
  );

  const minutes = useMemo(
    () => (docContent ? readingTime(docContent.content) : 0),
    [docContent]
  );

  // ── Not admin ─────────────────────────────────────────────────────────────

  if (!isAdmin) {
    return (
      <div className="content__inner">
        <div className="docs-empty" style={{ height: '60vh' }}>
          <div className="docs-empty__icon">
            <Icon name="book" size={36} />
          </div>
          <div className="docs-empty__title">Chỉ admin xem được tài liệu.</div>
          <span>Vui lòng đăng nhập bằng tài khoản admin.</span>
        </div>
      </div>
    );
  }

  // ── Doc list loading ──────────────────────────────────────────────────────

  if (docsLoading) {
    return (
      <div className="content__inner">
        <div className="docs-empty" style={{ height: '60vh' }}>
          <div className="docs-empty__icon">
            <Icon name="clock" size={32} />
          </div>
          <span>Đang tải danh sách tài liệu…</span>
        </div>
      </div>
    );
  }

  if (docsError === 'forbidden') {
    return (
      <div className="content__inner">
        <div className="docs-empty" style={{ height: '60vh' }}>
          <div className="docs-empty__icon">
            <Icon name="layers" size={36} />
          </div>
          <div className="docs-empty__title">Chỉ admin xem được tài liệu.</div>
        </div>
      </div>
    );
  }

  if (docsError) {
    return (
      <div className="content__inner">
        <div className="docs-empty" style={{ height: '60vh' }}>
          <div className="docs-empty__icon">
            <Icon name="layers" size={36} />
          </div>
          <div className="docs-empty__title">Không thể tải tài liệu.</div>
          <span>Kiểm tra kết nối và thử lại.</span>
        </div>
      </div>
    );
  }

  if (docs.length === 0) {
    return (
      <div className="content__inner">
        <div className="docs-empty" style={{ height: '60vh' }}>
          <div className="docs-empty__icon">
            <Icon name="book" size={36} />
          </div>
          <div className="docs-empty__title">Chưa có tài liệu.</div>
          <span>Thêm file markdown vào thư mục docs/ của dự án.</span>
        </div>
      </div>
    );
  }

  // ── Reading room layout ───────────────────────────────────────────────────

  return (
    <div className="docs-root">
      {/* ── INDEX column ── */}
      <aside className="docs-index">
        <div className="docs-index__search">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.5" y2="16.5"/>
          </svg>
          <input
            placeholder="Tìm tài liệu…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Tìm kiếm tài liệu"
          />
        </div>
        <div className="docs-index__list" role="list">
          {groups.length === 0 && (
            <div style={{ padding: '16px 14px', fontSize: 13, color: 'var(--text-3)' }}>
              Không tìm thấy tài liệu nào.
            </div>
          )}
          {groups.map(([group, items]) => (
            <div key={group} role="group">
              <div className="docs-index__group-heading">{group}</div>
              {items.map((doc) => (
                <button
                  key={doc.path}
                  className={`docs-index__item${selectedPath === doc.path ? ' active' : ''}`}
                  onClick={() => setSelectedPath(doc.path)}
                  role="listitem"
                  aria-current={selectedPath === doc.path ? 'page' : undefined}
                >
                  <span className="docs-index__item-title">{doc.title}</span>
                </button>
              ))}
            </div>
          ))}
        </div>
      </aside>

      {/* ── TOC Rail (left, next to index) ── */}
      <TocRail items={tocItems} readerRef={readerRef} />

      {/* ── Mobile doc selector ── */}
      <div className="docs-mobile-bar">
        <span className="docs-mobile-bar__label">Tài liệu:</span>
        <select
          value={selectedPath ?? ''}
          onChange={(e) => setSelectedPath(e.target.value || null)}
          aria-label="Chọn tài liệu"
        >
          {docs.map((d) => (
            <option key={d.path} value={d.path}>
              {d.group} / {d.title}
            </option>
          ))}
        </select>
      </div>

      {/* ── READING column ── */}
      <main className="docs-reader" ref={readerRef}>
        <div className="docs-reader__inner">
          {!selectedPath && (
            <div className="docs-empty">
              <div className="docs-empty__icon">
                <Icon name="book" size={40} />
              </div>
              <div className="docs-empty__title">Chọn một tài liệu để bắt đầu đọc.</div>
            </div>
          )}

          {selectedPath && contentLoading && <DocSkeleton />}

          {selectedPath && !contentLoading && docContent && (
            <>
              <header className="docs-header">
                <div className="docs-header__path">{docContent.path}</div>
                {currentMeta && (
                  <div className="docs-header__chips">
                    <span className="docs-header__chip">{currentMeta.group}</span>
                  </div>
                )}
                <h1 className="docs-header__title">{docContent.title}</h1>
                <div className="docs-header__meta">
                  {currentMeta?.group ?? ''} · {minutes} phút đọc
                </div>
              </header>

              <article className="docs-body">
                <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  rehypePlugins={[rehypeHighlight]}
                  components={MD_COMPONENTS as Record<string, unknown>}
                >
                  {docContent.content}
                </ReactMarkdown>
              </article>
            </>
          )}

          {selectedPath && !contentLoading && !docContent && (
            <div className="docs-empty">
              <div className="docs-empty__icon">
                <Icon name="layers" size={36} />
              </div>
              <div className="docs-empty__title">Không thể tải tài liệu này.</div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
