import { useState } from 'react';
import { Icon } from '../components/ui';

export default function Guide() {
  const [active, setActive] = useState('start');

  const sections = [
    { id: 'start',       label: 'Bắt đầu',      icon: 'play' },
    { id: 'dashboard',   label: 'Tổng quan',     icon: 'grid' },
    { id: 'stocks',      label: 'Cổ phiếu',      icon: 'candles' },
    { id: 'predictions', label: 'Dự đoán',       icon: 'pulse' },
    { id: 'training',    label: 'Huấn luyện',    icon: 'cpu' },
    { id: 'gold',        label: 'Giá vàng',      icon: 'gold' },
    { id: 'api',         label: 'API & Trigger', icon: 'refresh' },
  ];

  return (
    <div className="content__inner">
      <div style={{ display: 'grid', gridTemplateColumns: '220px 1fr', gap: 20, alignItems: 'start' }}>
        <div className="panel" style={{ position: 'sticky', top: 16 }}>
          <div className="panel__head"><div className="panel__title">Mục lục</div></div>
          <div className="panel__body" style={{ padding: '6px 0' }}>
            {sections.map((s) => (
              <button key={s.id} onClick={() => setActive(s.id)} style={{
                display: 'flex', alignItems: 'center', gap: 9, width: '100%',
                padding: '8px 14px', border: 'none', background: active === s.id ? 'var(--bg-3)' : 'transparent',
                color: active === s.id ? 'var(--accent)' : 'var(--text-2)',
                cursor: 'pointer', fontSize: 13, fontFamily: 'var(--font-ui)',
                borderLeft: active === s.id ? '2px solid var(--accent)' : '2px solid transparent',
                textAlign: 'left',
              }}>
                <Icon name={s.icon} size={15} />{s.label}
              </button>
            ))}
          </div>
        </div>
        <div>
          {active === 'start'       && <GuideStart />}
          {active === 'dashboard'   && <GuideDashboard />}
          {active === 'stocks'      && <GuideStocks />}
          {active === 'predictions' && <GuidePredictions />}
          {active === 'training'    && <GuideTraining />}
          {active === 'gold'        && <GuideGold />}
          {active === 'api'         && <GuideAPI />}
        </div>
      </div>
    </div>
  );
}

function GSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="panel" style={{ marginBottom: 16 }}>
      <div className="panel__head"><div className="panel__title">{title}</div></div>
      <div className="panel__body" style={{ lineHeight: 1.7, color: 'var(--text-2)', fontSize: 13 }}>{children}</div>
    </div>
  );
}

function GTip({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ background: 'var(--bg-3)', borderLeft: '3px solid var(--accent)', padding: '10px 14px', borderRadius: 4, margin: '10px 0', fontSize: 12, color: 'var(--text-2)' }}>
      <strong style={{ color: 'var(--accent)' }}>Tip: </strong>{children}
    </div>
  );
}

function GList({ items }: { items: React.ReactNode[] }) {
  return (
    <ul style={{ margin: '8px 0', paddingLeft: 20 }}>
      {items.map((it, i) => <li key={i} style={{ marginBottom: 4 }}>{it}</li>)}
    </ul>
  );
}

function GCode({ children }: { children: string }) {
  return (
    <code style={{ display: 'block', background: 'var(--bg-3)', border: '1px solid var(--border)', padding: '10px 14px', borderRadius: 4, fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text)', margin: '8px 0', whiteSpace: 'pre' }}>
      {children}
    </code>
  );
}

function GBadge({ color, children }: { color: string; children: React.ReactNode }) {
  const colors: Record<string, string> = { green: '#2FB57C', red: '#E05B5B', yellow: '#C9A23F', blue: '#5B8DEF', gray: 'var(--text-3)' };
  const c = colors[color] || colors.gray;
  return (
    <span style={{ background: c + '22', color: c, border: `1px solid ${c}44`, padding: '1px 7px', borderRadius: 3, fontSize: 11, fontFamily: 'var(--font-mono)' }}>
      {children}
    </span>
  );
}

function GuideStart() {
  return (
    <>
      <GSection title="Chào mừng đến với VNStock Terminal">
        <p><strong style={{ color: 'var(--text)' }}>VNStock Terminal</strong> là hệ thống phân tích và dự đoán giá cổ phiếu thị trường Việt Nam. Dữ liệu được thu thập tự động từ VietStock, sau đó chạy qua 3 thuật toán Machine Learning để dự đoán xu hướng giá.</p>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12, margin: '16px 0' }}>
          {[
            { icon: 'candles', title: 'Thu thập dữ liệu', desc: 'Crawl tự động hàng ngày lúc 12:00 từ VietStock cho toàn bộ VN30.' },
            { icon: 'cpu',     title: 'Huấn luyện ML',   desc: '3 thuật toán: VWMA, LSTM Neural Network, ARIMA-GARCH chạy mỗi Chủ nhật.' },
            { icon: 'pulse',   title: 'Dự đoán giá',     desc: 'Dự đoán giá đóng cửa hàng ngày lúc 18:00, so sánh với giá thực tế.' },
          ].map((c) => (
            <div key={c.title} style={{ background: 'var(--bg-3)', border: '1px solid var(--border)', borderRadius: 6, padding: 14 }}>
              <div style={{ color: 'var(--accent)', marginBottom: 6 }}><Icon name={c.icon} size={18} /></div>
              <div style={{ color: 'var(--text)', fontWeight: 600, marginBottom: 4, fontSize: 13 }}>{c.title}</div>
              <div style={{ fontSize: 12, color: 'var(--text-3)', lineHeight: 1.6 }}>{c.desc}</div>
            </div>
          ))}
        </div>
      </GSection>
      <GSection title="Điều hướng">
        <p>Dùng thanh sidebar bên trái hoặc bottom nav (mobile) để chuyển trang:</p>
        <GList items={[
          <><strong>Tổng quan</strong> — snapshot toàn thị trường: VN30, top tăng/giảm, độ chính xác thuật toán</>,
          <><strong>Cổ phiếu</strong> — xem biểu đồ nến, lịch sử giá từng mã trong VN30</>,
          <><strong>Dự đoán</strong> — danh sách dự đoán giá, so sánh thuật toán, phân tích sai số</>,
          <><strong>Huấn luyện</strong> — trạng thái và lịch sử các phiên training mô hình ML</>,
          <><strong>Giá vàng</strong> — theo dõi SJC và XAU/USD kèm dự đoán</>,
        ]} />
        <GTip>Nhấn phím <kbd style={{ background: 'var(--bg-3)', border: '1px solid var(--border)', padding: '1px 5px', borderRadius: 3 }}>/</kbd> để focus vào ô tìm kiếm mã cổ phiếu.</GTip>
      </GSection>
      <GSection title="Cài đặt giao diện">
        <p>Nhấn nút <strong>Tweaks</strong> (góc dưới bên phải) để tuỳ chỉnh:</p>
        <GList items={[
          'Màu nhấn (accent color) — 6 lựa chọn',
          'Mật độ hiển thị — compact / regular / comfy',
          'Cỡ chữ — 90% đến 115%',
          'Ẩn/hiện thanh ticker giá chạy ngang',
        ]} />
        <GTip>Chế độ tối/sáng có thể chuyển nhanh bằng 2 nút mặt trời / mặt trăng ở Topbar.</GTip>
      </GSection>
    </>
  );
}

function GuideDashboard() {
  return (
    <>
      <GSection title="Trang Tổng quan">
        <p>Dashboard cung cấp cái nhìn nhanh toàn thị trường và hiệu suất hệ thống dự đoán.</p>
      </GSection>
      <GSection title="Các thành phần chính">
        <GList items={[
          <><strong>KPI Cards</strong> — Số mã đang theo dõi, dự đoán hôm nay, độ chính xác trung bình, giá vàng SJC.</>,
          <><strong>Top Gainers / Losers</strong> — 5 mã tăng mạnh nhất và 5 mã giảm mạnh nhất phiên gần nhất.</>,
          <><strong>Biểu đồ VN30</strong> — Đường giá chỉ số VN30 theo thời gian.</>,
          <><strong>Độ chính xác thuật toán</strong> — So sánh hiệu suất 3 thuật toán theo thanh ngang.</>,
          <><strong>Dự đoán gần đây</strong> — Danh sách ngắn các dự đoán mới nhất kèm trạng thái xác nhận.</>,
        ]} />
      </GSection>
      <GSection title="Trạng thái dữ liệu">
        <p>Góc dưới sidebar hiển thị trạng thái nguồn dữ liệu:</p>
        <GList items={[
          <><GBadge color="green">● Dữ liệu trực tiếp</GBadge> — đang kết nối được API, dữ liệu là thật</>,
          <><GBadge color="yellow">● Dữ liệu mẫu</GBadge> — không kết nối được backend, hiển thị demo data</>,
          <><GBadge color="gray">● Đang tải…</GBadge> — đang fetch dữ liệu từ server</>,
        ]} />
        <GTip>Nếu thấy "Dữ liệu mẫu", hãy kiểm tra backend có đang chạy trên port 31300 không.</GTip>
      </GSection>
    </>
  );
}

function GuideStocks() {
  return (
    <>
      <GSection title="Trang Cổ phiếu">
        <p>Xem chi tiết giá và biểu đồ kỹ thuật cho từng mã trong rổ VN30.</p>
      </GSection>
      <GSection title="Cách sử dụng">
        <GList items={[
          'Chọn mã cổ phiếu từ danh sách bên trái (tìm kiếm hoặc cuộn).',
          'Biểu đồ nến (candlestick) hiển thị OHLC — Open, High, Low, Close.',
          'Chọn khung thời gian: 1T / 3T / 6T / 1N / Tất cả.',
          'Thanh volume phía dưới biểu đồ thể hiện khối lượng giao dịch.',
          'Panel chi tiết bên phải hiển thị: giá hiện tại, % thay đổi, P/E, vốn hoá.',
        ]} />
        <GTip>Hover chuột lên biểu đồ để xem giá OHLC chi tiết của từng phiên.</GTip>
      </GSection>
      <GSection title="Chỉ báo kỹ thuật">
        <GList items={[
          <><strong>MA20 / MA50</strong> — Đường trung bình động 20 và 50 phiên.</>,
          <><strong>Bollinger Bands</strong> — Dải biến động giá ± 2 độ lệch chuẩn.</>,
          <><strong>RSI</strong> — Chỉ số sức mạnh tương đối. {'>'}70 = quá mua, {'<'}30 = quá bán.</>,
          <><strong>Volume</strong> — So sánh khối lượng phiên hiện tại với trung bình 20 phiên.</>,
        ]} />
      </GSection>
    </>
  );
}

function GuidePredictions() {
  return (
    <>
      <GSection title="Trang Dự đoán">
        <p>Xem kết quả dự đoán giá đóng cửa của các thuật toán ML, đối chiếu với giá thực tế.</p>
      </GSection>
      <GSection title="Trạng thái dự đoán">
        <GList items={[
          <><GBadge color="yellow">pending</GBadge> — Dự đoán đã tạo, chưa đến ngày kiểm chứng.</>,
          <><GBadge color="green">confirmed</GBadge> — Dự đoán đúng hướng (sai số trong ngưỡng cho phép).</>,
          <><GBadge color="red">wrong</GBadge> — Dự đoán sai hướng hoặc vượt ngưỡng sai số.</>,
        ]} />
        <GTip>Độ chính xác được tính là tỉ lệ dự đoán "confirmed" trên tổng số đã xác nhận.</GTip>
      </GSection>
      <GSection title="3 Thuật toán">
        <GList items={[
          <><strong>VWMA (Moving Average)</strong> — Trung bình động có trọng số theo khối lượng. Nhanh, ổn định, phù hợp xu hướng dài hạn.</>,
          <><strong>LSTM Neural Network</strong> — Mạng nơ-ron hồi tiếp, học từ chuỗi thời gian dài. Tốt cho dữ liệu có mẫu lặp lại.</>,
          <><strong>ARIMA-GARCH</strong> — Mô hình thống kê kết hợp xu hướng (ARIMA) và biến động (GARCH). Tốt khi thị trường ổn định.</>,
        ]} />
      </GSection>
    </>
  );
}

function GuideTraining() {
  return (
    <>
      <GSection title="Trang Huấn luyện">
        <p>Theo dõi trạng thái và lịch sử các phiên huấn luyện mô hình Machine Learning.</p>
      </GSection>
      <GSection title="Lịch huấn luyện tự động">
        <GList items={[
          <><strong>Dự đoán hàng ngày</strong> — 18:00 mỗi ngày, chạy cả 3 thuật toán cho toàn bộ VN30.</>,
          <><strong>Huấn luyện lại</strong> — 09:00 sáng Chủ nhật, tái huấn luyện LSTM và ARIMA-GARCH với dữ liệu mới nhất.</>,
          <><strong>Crawl dữ liệu</strong> — 12:00 hàng ngày, thu thập giá từ VietStock.</>,
        ]} />
        <GTip>Có thể trigger thủ công bất kỳ lúc nào từ API (xem mục API & Trigger).</GTip>
      </GSection>
    </>
  );
}

function GuideGold() {
  return (
    <>
      <GSection title="Trang Giá vàng">
        <p>Theo dõi giá vàng trong nước và quốc tế, kèm dự đoán xu hướng.</p>
      </GSection>
      <GSection title="Nguồn dữ liệu">
        <GList items={[
          <><strong>Vàng SJC</strong> — Giá mua/bán vàng miếng SJC từ BTMC, crawl lúc 10:00 hàng ngày.</>,
          <><strong>XAU/USD</strong> — Giá vàng quốc tế tính theo USD/oz, cập nhật hàng ngày.</>,
        ]} />
      </GSection>
    </>
  );
}

function GuideAPI() {
  return (
    <>
      <GSection title="API & Trigger thủ công">
        <p>Hệ thống cung cấp API để kích hoạt crawler và dự đoán theo yêu cầu (không cần chờ cron).</p>
        <GTip>Các endpoint trigger yêu cầu <strong>API Key</strong> trong header <code>X-API-Key</code> (xem file <code>.env</code>).</GTip>
      </GSection>
      <GSection title="Trigger endpoints">
        <GCode>{`# Chạy crawler ngay (cần API Key)
curl -X POST http://localhost:31300/api/trigger/crawler \\
     -H "X-API-Key: YOUR_KEY"

# Chạy dự đoán ngay (cần API Key)
curl -X POST http://localhost:31300/api/trigger/predict \\
     -H "X-API-Key: YOUR_KEY"

# Huấn luyện mô hình ngay
curl -X POST http://localhost:31300/api/trigger/train

# Crawl giá vàng ngay
curl -X POST http://localhost:31300/api/trigger/gold-crawler`}</GCode>
      </GSection>
      <GSection title="Một số API đọc dữ liệu">
        <GCode>{`# Tổng quan thị trường
GET /api/market/overview?exchange=HOSE&sector=ngan-hang

# Lịch sử giá cổ phiếu
GET /api/stocks/VNM/history?from=2024-01-01&to=2024-12-31

# Giá vàng mới nhất
GET /api/gold/latest

# Thống kê dashboard
GET /api/dashboard/stats`}</GCode>
      </GSection>
      <GSection title="Health check">
        <GCode>{`GET http://localhost:31300/health
GET http://localhost:31300/health/simple
GET http://localhost:31300/health/ready`}</GCode>
      </GSection>
    </>
  );
}
