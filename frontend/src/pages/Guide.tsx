import { useState } from 'react';
import { Icon } from '../components/ui';
import { useLanguage } from '../context/LangContext';

export default function Guide() {
  const [active, setActive] = useState('start');
  const { t } = useLanguage();

  const sections = [
    { id: 'start',       label: t.guide.sections.start,       icon: 'play' },
    { id: 'markets',     label: t.guide.sections.markets,     icon: 'candles' },
    { id: 'predictions', label: t.guide.sections.predictions, icon: 'pulse' },
    { id: 'training',    label: t.guide.sections.training,    icon: 'cpu' },
    { id: 'simulation',  label: t.guide.sections.simulation,  icon: 'grid' },
    { id: 'monitoring',  label: t.guide.sections.monitoring,  icon: 'refresh' },
    { id: 'api',         label: t.guide.sections.api,         icon: 'refresh' },
  ];

  return (
    <div className="content__inner">
      <div style={{ display: 'grid', gridTemplateColumns: '220px 1fr', gap: 20, alignItems: 'start' }}>
        <div className="panel" style={{ position: 'sticky', top: 16 }}>
          <div className="panel__head"><div className="panel__title">{t.guide.tableOfContents}</div></div>
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
          {active === 'markets'     && <GuideMarkets />}
          {active === 'predictions' && <GuidePredictions />}
          {active === 'training'    && <GuideTraining />}
          {active === 'simulation'  && <GuideSimulation />}
          {active === 'monitoring'  && <GuideMonitoring />}
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
        <p><strong style={{ color: 'var(--text)' }}>VNStock Terminal</strong> là hệ thống phân tích và dự đoán giá tài sản tài chính. Dữ liệu được thu thập tự động từ nhiều nguồn, sau đó chạy qua <strong>11 thuật toán Machine Learning</strong> để dự đoán xu hướng giá.</p>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 12, margin: '16px 0' }}>
          {[
            { icon: 'candles', title: '4 thị trường', desc: 'Vàng SJC/XAU, NASDAQ 100 (15 mã), Crypto BTC/ETH/SOL, S&P 500 (16 mã).' },
            { icon: 'cpu',     title: '11 thuật toán ML',   desc: 'Moving Average, EMA/MACD, LSTM, GRU, ARIMA-GARCH, EGARCH, SARIMA, LightGBM, XGBoost, Random Forest, Ensemble.' },
            { icon: 'pulse',   title: 'Dự đoán tự động',   desc: 'Crawl dữ liệu → train định kỳ → sinh dự đoán → đối chiếu với giá thực tế (reconcile).' },
            { icon: 'grid',    title: 'Simulation bots',   desc: 'Bot giao dịch tự động chạy backtest và live theo từng thuật toán, leaderboard win-rate & P&L.' },
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
        <p>Dùng thanh sidebar bên trái để chuyển trang:</p>
        <GList items={[
          <><strong>Tổng quan</strong> — snapshot tổng hợp: dự đoán mới nhất, pipeline status, direction accuracy, top bots.</>,
          <><strong>Thị trường → Vàng / NASDAQ / Crypto / S&P 500</strong> — biểu đồ giá, dự đoán, lịch sử training.</>,
          <><strong>Simulation</strong> — leaderboard bots theo win-rate và return %.</>,
          <><strong>Giám sát dữ liệu</strong> — freshness crawl theo từng thị trường, trạng thái thuật toán, bots summary.</>,
          <><strong>Cài đặt</strong> — lịch cron, trigger thủ công, đổi mật khẩu.</>,
          <><strong>Quản lý người dùng / Nhóm thị trường</strong> — chỉ hiển thị với role admin/super_admin.</>,
        ]} />
        <GTip>Bấm nút <strong>Tweaks</strong> (góc dưới phải) để tuỳ chỉnh màu nhấn, mật độ, cỡ chữ và tắt/bật ticker.</GTip>
      </GSection>
      <GSection title="Đăng nhập">
        <p>Hầu hết tính năng yêu cầu đăng nhập. Hệ thống có 3 vai trò:</p>
        <GList items={[
          <><GBadge color="blue">super_admin</GBadge> — toàn quyền, truy cập tất cả 4 thị trường.</>,
          <><GBadge color="yellow">admin</GBadge> — quản lý users và market groups, truy cập markets được gán.</>,
          <><GBadge color="gray">user</GBadge> — chỉ truy cập markets được gán qua market groups.</>,
        ]} />
        <GTip>Nếu không thấy một thị trường trong menu, liên hệ admin để được thêm vào market group.</GTip>
      </GSection>
      <GSection title="Cài đặt giao diện">
        <GList items={[
          'Màu nhấn (accent color) — 6 lựa chọn',
          'Mật độ hiển thị — compact / regular / comfy',
          'Cỡ chữ — 90% đến 115%',
          'Ẩn/hiện thanh ticker giá chạy ngang',
        ]} />
        <GTip>Chế độ tối/sáng chuyển nhanh bằng 2 nút mặt trời / mặt trăng ở Topbar.</GTip>
      </GSection>
    </>
  );
}

function GuideMarkets() {
  return (
    <>
      <GSection title="4 thị trường được theo dõi">
        <p>Mỗi thị trường có trang riêng với 3 tab: <strong>Tổng quan</strong> (biểu đồ giá + dự đoán), <strong>Dự đoán</strong> (bảng phân trang), <strong>Huấn luyện</strong> (lịch sử training sessions).</p>
      </GSection>
      <GSection title="Vàng (Gold)">
        <GList items={[
          <><strong>Nguồn dữ liệu:</strong> BTMC API (vàng miếng SJC 1 lượng), Yahoo Finance (XAU/USD).</>,
          <><strong>Lịch crawl:</strong> Mỗi giờ, liên tục 24/7 — vàng không bị ảnh hưởng bởi giờ thị trường.</>,
          <><strong>Đơn vị giá:</strong> VND (SJC) và USD/oz (XAU).</>,
          'Bảng so sánh giá mua/bán nhiều nhà cung cấp: BTMC, Phú Quý.',
        ]} />
      </GSection>
      <GSection title="NASDAQ 100">
        <GList items={[
          <><strong>15 mã theo dõi:</strong> AAPL, MSFT, GOOGL, AMZN, NVDA, META, TSLA, AVGO, COST, NFLX, AMD, QCOM, INTC, ADBE, MU.</>,
          <><strong>Nguồn:</strong> Yahoo Finance.</>,
          <><strong>Lịch crawl:</strong> Mỗi giờ phút 15, chỉ thứ Hai đến thứ Sáu.</>,
          <><strong>Đóng cửa:</strong> Cuối tuần và ngày lễ NYSE — banner cảnh báo hiển thị tự động.</>,
        ]} />
        <GTip>Khi thị trường đóng cửa, crawler, dự đoán và bot tự động bị tạm dừng đến phiên kế tiếp.</GTip>
      </GSection>
      <GSection title="Cryptocurrency">
        <GList items={[
          <><strong>3 coin:</strong> BTC (Bitcoin), ETH (Ethereum), SOL (Solana).</>,
          <><strong>Nguồn:</strong> CoinGecko API.</>,
          <><strong>Lịch crawl:</strong> Mỗi 2 giờ, liên tục 24/7.</>,
          'Giá tính theo USD; không bị ảnh hưởng bởi ngày lễ.',
        ]} />
      </GSection>
      <GSection title="S&P 500">
        <GList items={[
          <><strong>16 mã theo dõi:</strong> SPY, QQQ, JPM, BAC, GS, JNJ, UNH, PFE, PG, KO, WMT, XOM, CVX, V, MA (+ 1 mã khác).</>,
          <><strong>Nguồn:</strong> Yahoo Finance.</>,
          <><strong>Lịch crawl:</strong> Mỗi giờ phút 0 và 30, chỉ thứ Hai đến thứ Sáu.</>,
          <><strong>Đóng cửa:</strong> Cuối tuần và ngày lễ NYSE — tương tự NASDAQ.</>,
        ]} />
      </GSection>
      <GSection title="Đọc biểu đồ">
        <GList items={[
          'Đường xanh = giá thực tế; đường màu nhấn = dự đoán Ensemble.',
          'Hover chuột để xem giá chi tiết từng ngày.',
          'Dùng bộ lọc thời gian (Today / 7D / 30D / 90D / 1Năm) để thu/phóng.',
          'Tab "Chi tiết" so sánh direction accuracy từng thuật toán theo khoảng thời gian.',
        ]} />
      </GSection>
    </>
  );
}

function GuidePredictions() {
  return (
    <>
      <GSection title="Trang Dự đoán">
        <p>Mỗi thị trường có tab <strong>Dự đoán</strong> (từ route <code>/markets/&lt;key&gt;/predictions</code>) hiển thị bảng phân trang toàn bộ dự đoán và kết quả đối chiếu.</p>
      </GSection>
      <GSection title="Trạng thái dự đoán">
        <GList items={[
          <><GBadge color="yellow">Đang chờ</GBadge> — dự đoán đã tạo, chưa đến ngày kiểm chứng (target_date chưa qua).</>,
          <><GBadge color="green">Chính xác</GBadge> — hướng dự đoán (tăng/giảm) khớp với giá thực tế.</>,
          <><GBadge color="blue">Gần đúng</GBadge> — sai số nhỏ nhưng hướng đúng.</>,
          <><GBadge color="red">Sai lệch</GBadge> — hướng hoặc biên độ dự đoán không khớp.</>,
        ]} />
        <GTip>Reconcile tự động chạy lúc 6:00 sáng mỗi ngày để cập nhật trạng thái dự đoán với giá thực tế.</GTip>
      </GSection>
      <GSection title="11 Thuật toán">
        <GList items={[
          <><strong>moving_average</strong> — VWMA trend slope + RSI momentum + StochRSI overlay.</>,
          <><strong>ema</strong> — EMA slope + MACD momentum boost + Bollinger %B mean-reversion.</>,
          <><strong>lstm_nn</strong> — PyTorch LSTM 2 lớp, hidden=64, sequence=60 phiên.</>,
          <><strong>gru_nn</strong> — PyTorch GRU 2 lớp, hidden=64, sequence=60 phiên.</>,
          <><strong>arima_garch</strong> — ARIMA(2,1,2) + GARCH(1,1) cho xu hướng + biến động.</>,
          <><strong>egarch</strong> — EGARCH(1,1) với HARX mean model.</>,
          <><strong>sarima</strong> — SARIMA seasonal period 5 (tuần giao dịch).</>,
          <><strong>lightgbm</strong> — LightGBM ~30 features, Optuna hyperopt 30 trials.</>,
          <><strong>xgboost</strong> — XGBoost ~30 features, Optuna hyperopt 30 trials.</>,
          <><strong>random_forest</strong> — RandomForest ~30 features, n_estimators=200.</>,
          <><strong>ensemble</strong> — Trung bình đồng đều của 10 thuật toán trên.</>,
        ]} />
      </GSection>
      <GSection title="Direction Accuracy">
        <p><strong>Direction accuracy</strong> là tỉ lệ dự đoán đúng hướng (tăng/giảm so với giá hiện tại). Đây là metric chính của hệ thống — quan trọng hơn sai số tuyệt đối.</p>
        <GList items={[
          'Hiển thị trên Dashboard (direction accuracy TB các thị trường).',
          'Xem chi tiết theo thuật toán ở tab "Chi tiết" của từng thị trường.',
          'Trang Giám sát hiển thị per-algo direction accuracy cho mỗi thị trường.',
        ]} />
      </GSection>
    </>
  );
}

function GuideTraining() {
  return (
    <>
      <GSection title="Pipeline tự động">
        <p>Mỗi thị trường có pipeline riêng: <strong>crawl → train (mỗi 10 lần crawl) → predict</strong>. Lịch cron được lưu trong DB và có thể chỉnh sửa live qua trang Cài đặt.</p>
        <GList items={[
          <><strong>Gold</strong> — Crawl mỗi giờ (phút 0); pipeline chạy 24/7.</>,
          <><strong>NASDAQ</strong> — Crawl mỗi giờ phút 15, thứ Hai–Sáu.</>,
          <><strong>S&P 500</strong> — Crawl mỗi giờ phút 0 và 30, thứ Hai–Sáu.</>,
          <><strong>Crypto</strong> — Crawl mỗi 2 giờ, 24/7.</>,
        ]} />
      </GSection>
      <GSection title="Training định kỳ">
        <GList items={[
          <><strong>Gold</strong> — Chủ nhật 3:00 AM.</>,
          <><strong>NASDAQ</strong> — Chủ nhật 4:00 AM.</>,
          <><strong>Crypto</strong> — Chủ nhật 5:00 AM.</>,
          <><strong>S&P 500</strong> — Chủ nhật 7:00 AM.</>,
          <><strong>Reconcile</strong> — Hàng ngày 6:00 AM, đối chiếu dự đoán với giá thực tế.</>,
        ]} />
        <GTip>Training tự động cũng được kích hoạt trong pipeline sau mỗi 10 lần crawl — không chỉ theo lịch tuần.</GTip>
      </GSection>
      <GSection title="Tab Huấn luyện (mỗi thị trường)">
        <p>Route <code>/markets/&lt;key&gt;/training</code> — hiển thị lịch sử các phiên training:</p>
        <GList items={[
          'Mỗi session có: thuật toán, thời gian bắt đầu/kết thúc, số mẫu, độ chính xác đạt được.',
          'Lọc theo thuật toán cụ thể hoặc xem tất cả.',
          'Phân trang — mỗi trang 20 sessions.',
        ]} />
      </GSection>
      <GSection title="Trigger thủ công">
        <p>Vào <strong>Cài đặt → Thao tác thủ công</strong> để trigger ngay lập tức (yêu cầu JWT role admin):</p>
        <GList items={[
          'Crawl + dự đoán từng thị trường.',
          'Huấn luyện tất cả thuật toán (train all).',
          'Reconcile thủ công.',
        ]} />
      </GSection>
    </>
  );
}

function GuideSimulation() {
  return (
    <>
      <GSection title="Simulation Bots">
        <p>Hệ thống chạy các bot giao dịch ảo để đánh giá hiệu suất thực tế của từng thuật toán trên dữ liệu lịch sử (backtest) và live.</p>
      </GSection>
      <GSection title="Leaderboard">
        <p>Trang <strong>Simulation</strong> hiển thị bảng xếp hạng tất cả bots theo:</p>
        <GList items={[
          <><strong>Return %</strong> — lợi nhuận/lỗ so với vốn ban đầu.</>,
          <><strong>Win Rate</strong> — tỉ lệ giao dịch thắng.</>,
          <><strong>Profit Factor</strong> — tổng lợi nhuận / tổng lỗ.</>,
          'Lọc theo thị trường, thuật toán.',
          'Nhấn vào một bot để xem chi tiết portfolio chart và lịch sử giao dịch.',
        ]} />
        <GTip>Dashboard cũng hiển thị Top 3 bots hiệu suất cao nhất — bấm "Xem tất cả →" để vào Leaderboard.</GTip>
      </GSection>
      <GSection title="Bot detail">
        <p>Trang <code>/simulation/&lt;botId&gt;</code> hiển thị:</p>
        <GList items={[
          'Biểu đồ giá trị danh mục theo thời gian.',
          'Cấu hình bot: thị trường, thuật toán, vốn ban đầu, chiến lược.',
          'Lịch sử giao dịch phân trang (loại BUY/SELL, giá, khối lượng, tín hiệu).',
        ]} />
      </GSection>
      <GSection title="Live mode & Reset">
        <p>Ngoài backtest lịch sử, mỗi bot có thể chạy <strong>live step</strong> — thực thi tín hiệu giao dịch dựa trên dự đoán mới nhất (trigger từ Cài đặt hoặc lịch cron 8PM hàng ngày). Admin có thể <strong>Reset</strong> tất cả bots về trạng thái mới.</p>
      </GSection>
    </>
  );
}

function GuideMonitoring() {
  return (
    <>
      <GSection title="Trang Giám sát dữ liệu">
        <p>Route <code>/monitoring</code> — cung cấp cái nhìn toàn diện về sức khoẻ của data pipeline. Yêu cầu đăng nhập. Dữ liệu cache 30 giây.</p>
      </GSection>
      <GSection title="Freshness crawl (4 thị trường)">
        <p>Mỗi market card hiển thị:</p>
        <GList items={[
          <><GBadge color="green">Tươi</GBadge> / <GBadge color="red">Cũ</GBadge> — data coi là cũ nếu lần crawl cuối hơn 3 giờ hoặc không có dữ liệu hôm nay.</>,
          'Thời điểm crawl cuối cùng (daily và intraday).',
          'Số bản ghi thu thập hôm nay.',
          <><GBadge color="yellow">Đóng cửa</GBadge> — hiển thị khi thị trường đang đóng (NASDAQ/SP500 cuối tuần/lễ).</>,
        ]} />
      </GSection>
      <GSection title="Trạng thái dự đoán">
        <p>Mỗi thị trường cũng hiển thị:</p>
        <GList items={[
          'Thời điểm dự đoán cuối cùng.',
          'Tổng số dự đoán hôm nay.',
          'Số thuật toán kỳ vọng vs số thực tế — <strong>thiếu thuật toán</strong> chỉ ra pipeline có vấn đề.',
          'Bảng chi tiết: mỗi thuật toán — số dự đoán hôm nay, direction accuracy, số đã reconcile, số đúng.',
        ]} />
        <GTip>Nếu một thuật toán thiếu trong bảng hôm nay, có thể crawl bị lỗi hoặc training chưa chạy. Vào Cài đặt để trigger thủ công.</GTip>
      </GSection>
      <GSection title="Tổng hợp Bot Trading">
        <p>Phần cuối trang Giám sát hiển thị:</p>
        <GList items={[
          'Số bots đang hoạt động, phân bố theo thị trường.',
          'Bảng đầy đủ tất cả bots: market, algo, số giao dịch thắng/thua, win rate, total P&L, return %.',
          'Có thể sắp xếp các cột để tìm bot hiệu suất cao/thấp nhất.',
        ]} />
      </GSection>
    </>
  );
}

function GuideAPI() {
  return (
    <>
      <GSection title="Xác thực (JWT)">
        <p>Tất cả API đều dùng <strong>JWT Bearer token</strong> — không dùng API key cũ. Lấy token bằng endpoint login:</p>
        <GCode>{`# Đăng nhập, lấy JWT token
curl -s -X POST http://localhost/api/auth/login \\
     -H "Content-Type: application/json" \\
     -d '{"username":"admin","password":"admin123"}' | jq -r '.token'`}</GCode>
        <GTip>Token hợp lệ trong 24 giờ. Dùng trong header: <code>Authorization: Bearer &lt;token&gt;</code>. Trigger endpoints yêu cầu role <GBadge color="yellow">admin</GBadge> hoặc <GBadge color="blue">super_admin</GBadge>.</GTip>
      </GSection>
      <GSection title="Trigger endpoints">
        <GCode>{`TOKEN=$(curl -s -X POST http://localhost/api/auth/login \\
  -H "Content-Type: application/json" \\
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# Crawl + dự đoán từng thị trường
curl -X POST http://localhost/api/trigger/gold-crawler    -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/gold-predict    -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/nasdaq-crawler  -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/nasdaq-predict  -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/crypto-crawler  -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/crypto-predict  -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/sp500-crawler   -H "Authorization: Bearer $TOKEN"
curl -X POST http://localhost/api/trigger/sp500-predict   -H "Authorization: Bearer $TOKEN"

# Huấn luyện
curl -X POST http://localhost/api/trigger/train           -H "Authorization: Bearer $TOKEN"

# Reconcile dự đoán với giá thực tế
curl -X POST http://localhost/api/trigger/reconcile       -H "Authorization: Bearer $TOKEN"`}</GCode>
      </GSection>
      <GSection title="Một số API đọc dữ liệu">
        <GCode>{`# Giá vàng mới nhất
GET /api/gold/latest

# Dự đoán vàng mới nhất
GET /api/gold/predictions/latest

# Giá Crypto mới nhất
GET /api/crypto/latest

# Dự đoán NASDAQ
GET /api/nasdaq/predictions/latest

# Thống kê dashboard
GET /api/dashboard/stats

# Direction accuracy (yêu cầu JWT)
GET /api/predictions/direction-accuracy?market=GOLD

# Monitoring overview (yêu cầu JWT)
GET /api/monitoring/overview

# Lịch cron
GET /api/schedules    -H "Authorization: Bearer $TOKEN"

# Cập nhật lịch cron
PUT /api/schedules/crawler_gold
    -H "Authorization: Bearer $TOKEN"
    -d '{"cron_expression":"0 0 * * * *","enabled":true}'`}</GCode>
      </GSection>
      <GSection title="Health check">
        <GCode>{`# Gateway health (không cần auth)
GET http://localhost/healthz
GET http://localhost/readyz

# API Backend trực tiếp (port 8118)
GET http://localhost:8118/health`}</GCode>
        <GTip>Swagger UI đầy đủ tại <code>http://localhost:8118/swagger/</code> — xem tất cả endpoints kèm schema request/response.</GTip>
      </GSection>
    </>
  );
}
