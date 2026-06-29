# Meta-Stacking — chiến thuật bot đọc tất cả dự đoán rồi mới quyết (giải thích từ số 0)

**Loại:** Chiến thuật giao dịch (trading tactic) — KHÔNG phải thuật toán dự đoán.
**Key bot:** `meta_stack` (pooled) / `meta_stack__ps` (per-symbol)
**Tầng:** simulation (`prediction-svc/src/simulation/meta_stack.py`)
**Stateful:** Có — huấn luyện một mô hình supervised và **lưu checkpoint** (`${RL_MODEL_DIR}/meta_{market}.pkl`, hoặc `meta_{market}_{symbol}.pkl` cho per-symbol)
**Đầu vào:** dự đoán của **tất cả** thuật toán trong `algos/` + lịch sử độ chính xác của chúng
**Đầu ra:** hành động `{BUY, SELL, HOLD}` + cỡ vị thế

> **Tài liệu này viết cho người chưa biết gì về machine learning quyết định.** Chỉ cần biết toán cơ bản: phần trăm thay đổi, trung bình, xác suất, vector. Mỗi thuật ngữ được **định nghĩa và giải thích trực giác trước**, công thức đến sau. Công thức render trên GitHub và bản xem trước Markdown của VSCode.

---

## Phần A. Trực giác: vấn đề là gì?

Hãy hình dung bạn có **11 nhà phân tích** (chính là 11 thuật toán dự đoán trong `algos/`). Mỗi giờ, mỗi người đưa ra một dự đoán: "giờ sau giá lên/xuống bao nhiêu phần trăm".

Bot **mặc định** hiện tại làm thế này: chọn **đúng một** nhà phân tích, và nghe lời anh ta một cách máy móc:

> *"Anh ta nói lên hơn 0.5% à? Mua. Anh ta nói xuống hơn 0.5%? Bán. Còn lại đứng im."*

Đây gọi là **luật ngưỡng (threshold rule)** trên **một** thuật toán. Ba vấn đề:

1. **Tin mù.** Bot không hề biết nhà phân tích này *tuần vừa rồi* đoán đúng hay sai. Có người đoán đúng 62%, có người đúng 25% (tệ hơn tung đồng xu!) — nhưng bot đối xử với họ y như nhau.
2. **Không hội ý.** Mười nhà phân tích còn lại nói gì? Bot không nghe. Nếu 9/11 người cùng nói "lên" thì độ tin phải cao hơn chứ?
3. **Ngưỡng cứng, vào lệnh full.** Cùng một ngưỡng 0.5% áp cho cả vàng (ít biến động) lẫn crypto (biến động mạnh) là vô lý. Và cứ vượt ngưỡng là vào *toàn bộ* vốn, bất kể tín hiệu mạnh hay yếu.

**Ý tưởng Meta-Stacking:** thay vì nghe một người, ta thuê một **trưởng nhóm**. Mỗi giờ, trưởng nhóm:

- Nghe **tất cả** dự đoán cùng lúc,
- Nhớ **gần đây ai đúng nhiều, ai sai nhiều** (và ai *sai một cách có hệ thống* — để làm ngược lại lời họ),
- Cân nhắc cả **độ biến động hiện tại** của thị trường,
- Rồi đưa ra **một con số duy nhất**: *xác suất giá sẽ tăng giờ sau*, và **đặt cược nhiều hay ít tuỳ độ chắc chắn**.

"Stacking" (xếp chồng) là thuật ngữ ML cho đúng việc này: **dùng đầu ra của nhiều mô hình làm đầu vào cho một mô hình cấp trên**. Phần còn lại của tài liệu biến câu chuyện "trưởng nhóm" này thành toán.

---

## Phần B. Vì sao có cơ sở để tin Meta-Stacking ăn được?

Một meta-model chỉ có ích nếu các nhà phân tích **khác nhau về độ tin cậy** — nếu ai cũng đúng 50% thì tổng hợp kiểu gì cũng 50%. Đo thực tế độ chính xác hướng (direction accuracy = tỉ lệ đoán đúng lên/xuống) trên dữ liệu đã đối chiếu (2026-06-27):

| Thị trường | Tốt nhất | Tệ nhất | Nhận xét |
|---|---|---|---|
| **GOLD** | egarch **62.5%** | lightgbm / xgboost / random_forest **~25%** | Phân tán **cực mạnh** |
| **CRYPTO** | rl_dqn 54.5% | ema 37.3% | egarch / moving_average ~54% |
| **NASDAQ** | ema 55.4% | gru 45.4% | Cụm quanh 50% |
| **SP500** | lightgbm 53.6% | egarch 45.3% | Cụm quanh 50% |

Ba điều rút ra:

1. **Đa số thuật toán ~45–55%** (gần như tung đồng xu) → đừng kỳ vọng meta thành cỗ máy in tiền; **biên lợi thế nhỏ**, và phí giao dịch theo giờ sẽ ăn mòn nó.
2. **Độ chính xác phân tán mạnh giữa các thuật toán** → *đây chính là nguyên liệu*. Một trưởng nhóm biết "tin egarch trên GOLD, bỏ qua phần còn lại" sẽ ăn đứt việc cho mỗi người một phiếu ngang nhau.
3. **GOLD lightgbm/xgboost/rf ~25% = sai có hệ thống** (tệ hơn ngẫu nhiên rõ rệt). Đây có thể là **mỏ vàng**: nếu một người luôn đoán ngược, ta chỉ việc **làm ngược lời họ** để ăn ~75%. Meta-model học được điều này tự động.

→ Kết luận: hướng đi có **bằng chứng định lượng** ủng hộ, không phải lý thuyết suông.

---

## Phần C. Bài toán học máy: phân loại nhị phân

### C.1 Ta học cái gì?

Meta-model **không** đoán giá. Nó trả lời đúng **một** câu hỏi nhị phân:

> *"Giờ sau, giá sẽ **tăng** hay **giảm** so với bây giờ?"*

Đây là **bài toán phân loại nhị phân (binary classification)**. Đầu ra mong muốn không phải "có/không" cứng, mà là một **xác suất**:

$$
p = P(\text{up} \mid x_t) \in [0,1],
$$

đọc là "xác suất giá tăng, khi biết thông tin $x_t$ tại thời điểm $t$". $p=0.5$ nghĩa là chịu, không biết; $p=0.8$ nghĩa là khá chắc giá lên.

### C.2 Vì sao tách rời quyết định khỏi vị thế?

Một lựa chọn thiết kế quan trọng: **mô hình chỉ đoán hướng thị trường, KHÔNG quan tâm bot đang giữ lệnh hay không.** Việc "đang giữ lệnh thì nên làm gì" để **lớp chính sách bot** (Phần G) xử lý.

Lợi ích: (1) nhãn huấn luyện đơn giản và khách quan (giá lên hay xuống, không phụ thuộc bot); (2) tránh rò rỉ thông tin; (3) cùng một mô hình P(up) dùng được cho nhiều luật giao dịch khác nhau.

---

## Phần D. Đặc trưng đầu vào $x_t$

Tại mỗi thời điểm $t$ cho một mã $j$, ta xây một **vector đặc trưng** $x_t$ gồm ba khối.

### D.1 Khối 1 — Dự đoán của từng thuật toán ($g_{t,a}$)

Với mỗi thuật toán $a \in \{1,\dots,A\}$ (A = số thuật toán có dự đoán), lấy giá nó dự đoán $\hat p_{t,a}$ và giá hiện tại $p_t$, tính **phần trăm thay đổi dự đoán**:

$$
g_{t,a} = \frac{\hat p_{t,a} - p_t}{p_t}.
$$

$g_{t,a} > 0$: thuật toán $a$ nghĩ giá lên; $g_{t,a} < 0$: nghĩ giá xuống. Đây là "ý kiến" của từng nhà phân tích, ở dạng số. Lấy từ các bảng `*_predictions`.

### D.2 Khối 2 — Độ tin cậy lịch sử của từng thuật toán ($w_{t,a}$)

Đây là khối **quan trọng nhất** — thứ biến egarch-62% thành tiếng nói to và lightgbm-GOLD-25% thành tiếng nói (bị đảo).

$w_{t,a}$ = **direction accuracy rolling** của thuật toán $a$ trên $K$ lần dự đoán **đã đối chiếu gần nhất** tính tới thời điểm $t$ (mặc định $K=40$):

$$
w_{t,a} = \frac{1}{K}\sum_{i=1}^{K} \mathbf{1}\!\left[\,\mathrm{sign}(\hat p_{a,(i)} - p_{(i)}) = \mathrm{sign}(p^{\text{actual}}_{(i)} - p_{(i)})\,\right],
$$

trong đó $\mathbf{1}[\cdot]=1$ nếu vế trong đúng (đoán đúng hướng), ngược lại 0; tổng chạy trên $K$ dự đoán gần nhất của thuật toán $a$ mà ta **đã biết kết quả thật**. Nếu chưa đủ $K$ mẫu → điền $w_{t,a}=0.5$ (trung lập).

$w_{t,a}$ chính là cột `direction_correct` (đã có sẵn trong DB) lấy trung bình trượt.

> **⚠️ Chống rò rỉ (leakage) — đọc kỹ.** Tổng trên **chỉ** được dùng các dự đoán có thời điểm chín (`target_date`) **trước** $t$. Nếu lỡ tính cả những lần mà kết quả thật chỉ lộ ra **sau** $t$, mô hình đang "nhìn trộm tương lai" → backtest đẹp giả tạo, ra thực tế sập. Đây là **rủi ro số 1** của toàn bộ chiến thuật (xem Phần H).

### D.3 Khối 3 — Đặc trưng regime (bối cảnh thị trường)

Hai dự đoán cùng nói "+0.3%" nhưng trong thị trường lặng khác hẳn trong thị trường bão. Ta thêm:

- **Độ biến động** $\sigma_t$ — độ lệch chuẩn của log-return trong cửa sổ 20 bước gần nhất:

$$
\sigma_t = \sqrt{\frac{1}{n-1}\sum_{k=t-n+1}^{t}\big(r_k - \bar r\big)^2}, \qquad r_k = \ln\frac{p_k}{p_{k-1}}, \quad n=20.
$$

- **Momentum ngắn** $\text{mom}_t$ — tổng vài return gần nhất (xu hướng tức thời).

Giữ khối regime **nhỏ** (chỉ 2 đặc trưng) để chống overfit — dữ liệu ít, thêm nhiều đặc trưng dễ học vẹt nhiễu.

### D.4 Ghép lại

$$
x_t = \big[\,\underbrace{g_{t,1},\dots,g_{t,A}}_{\text{ý kiến mỗi algo}}\ \big\Vert\ \underbrace{w_{t,1},\dots,w_{t,A}}_{\text{độ tin lịch sử mỗi algo}}\ \big\Vert\ \underbrace{\sigma_t,\ \text{mom}_t}_{\text{regime}}\,\big].
$$

Ký hiệu $\Vert$ = ghép nối vector. Chiều = $2A + 2$.

---

## Phần E. Nhãn $y_t$ — đáp án đúng

Với học **có giám sát**, mỗi $x_t$ cần một đáp án. Đáp án ở đây là: giờ sau giá thật **tăng** hay không.

$$
y_t = \mathbf{1}\!\left[\,p^{\text{actual}}_{t+1h} > p_t\,\right] \in \{0, 1\}.
$$

- $y_t = 1$: giá thật giờ sau cao hơn bây giờ (tăng).
- $y_t = 0$: bằng hoặc thấp hơn (giảm).

Giá thật $p^{\text{actual}}_{t+1h}$ lấy từ quá trình **reconcile** (đối chiếu) đã có sẵn. Nếu thiếu, suy ngược từ `direction_correct` của bất kỳ thuật toán nào: biết nó đoán hướng nào ($\mathrm{sign}(\hat p_a - p)$) và đoán đúng/sai (`direction_correct`) → suy ra hướng thật.

---

## Phần F. Mô hình & hiệu chỉnh xác suất

### F.1 LightGBM classifier với cân bằng lớp

Ta dùng **LightGBM** (gradient boosting trên cây quyết định, đã có trong dự án) làm bộ phân loại với `class_weight="balanced"`. Điều này quan trọng: nếu dữ liệu gần đây thiên về "giảm" (thị trường đang downtrend), mô hình không cân bằng sẽ học prior lệch và luôn ra $p \approx 0.33$ — bot long-only sẽ không bao giờ mua. Cân bằng lớp buộc mô hình phải học cả hai hướng, giữ $p$ xoay quanh 0.5.

Vì sao LightGBM mà không phải hồi quy tuyến tính? Vì quan hệ ở đây **phi tuyến và có tương tác**: ví dụ "tin $g_a$ **chỉ khi** $w_a$ cao **và** $\sigma$ thấp" — cây quyết định nắm bắt loại logic điều kiện này tự nhiên.

### F.2 Vì sao cần hiệu chỉnh (calibration)?

Xác suất thô $\tilde p$ từ LightGBM thường **không đáng tin về mặt con số**: khi mô hình nói "0.8", thực tế có thể chỉ tăng 65% số lần. Với bot, sai lệch này chí mạng vì ta **đặt cược theo $p$**.

**Calibration** sửa điều đó: học một hàm đơn điệu $h$ biến $\tilde p$ thô thành $p$ đã hiệu chỉnh. Ta dùng **sigmoid (Platt scaling)** — đơn điệu, bảo toàn spread của phân phối xác suất. Isotonic regression (hồi quy bậc thang) bị thay thế vì nó có xu hướng co p về base-rate của tập huấn luyện, đặc biệt khi dữ liệu ít, làm mọi p đều phẳng ~0.33 hay ~0.5:

$$
p = h(\tilde p), \qquad h \text{ sigmoid đơn điệu.}
$$

**Bỏ qua calibration** khi: (a) tập calib có < 30 mẫu — không đủ ổn định; hoặc (b) sau calibration, std của p trên tập test < 0.02 — vẫn phẳng, dùng raw balanced model còn tốt hơn.

### F.3 Đo chất lượng xác suất — Brier score

Để biết xác suất có tốt không, dùng **Brier score** = trung bình bình phương sai giữa xác suất dự báo và kết quả thật:

$$
\text{Brier} = \frac{1}{N}\sum_{i=1}^{N}\big(p_i - y_i\big)^2.
$$

Càng **thấp** càng tốt (0 = hoàn hảo, 0.25 = đoán bừa 0.5). Ta báo cáo Brier **trên dữ liệu out-of-sample** (Phần H), cùng direction accuracy.

---

## Phần G. Chính sách bot — từ xác suất ra hành động

Mô hình cho ra $p = P(\text{up})$ **cho từng mã**. Bây giờ bot biến chúng thành lệnh thật, tại hàm `_step_meta` — theo **2 lượt**:

```
SL/TP cứng trước (lưới an toàn — cắt lỗ/chốt lời, áp cho mọi bot)

--- Lượt 1: tính p cho TẤT CẢ mã có thể giao dịch của bước đó ---
cho mỗi mã j:
    p_j = P(up | x_{t,j})             # từ mô hình đã huấn luyện hoặc fallback Phần I

--- Lượt 2: ngưỡng thích ứng, quyết định ---
μ_p  = trung bình các p_j
σ_p  = độ lệch chuẩn các p_j
ngưỡng = max(0.5 + δ,  μ_p + K × σ_p)    K = 1.0 (hằng số module)

# Trường hợp đặc biệt: chỉ 1 mã (per-symbol bot hoặc thị trường 1 mã)
#   → σ_p không xác định → ngưỡng = 0.5 + δ  (chỉ sàn tuyệt đối)

cho mỗi mã j:
    nếu p_j > ngưỡng:
        BUY, với cỡ lệnh = clamp((p_j − 0.5)/0.5, 0, 1) × max_position_pct
    nếu p_j < 0.5 − δ  và  đang giữ vị thế mã j:
        SELL
    ngược lại:
        HOLD
```

**Ý nghĩa sàn tuyệt đối** `0.5+δ`: đảm bảo **không bao giờ** mua mã mà mô hình cho là có nhiều khả năng giảm — kể cả khi nó là mã "ít giảm nhất" trong một bước thị trường xuống đồng loạt. Nếu không mã nào vượt ngưỡng → ôm tiền mặt. Đây là hành vi đúng.

### G.1 Margin $\delta$ — chống lật lệnh

$\delta$ là vùng đệm quanh 0.5: chỉ hành động khi đủ tự tin, tránh mua-bán liên tục khi $p$ dập dình quanh 50/50 (mỗi lần lật lệnh tốn phí). Mặc định lấy lại từ config sẵn có: $\delta = \texttt{buy\_threshold}/100$.

### G.2 Conviction sizing — đặt cược theo độ chắc

Khác hẳn bot ngưỡng (luôn vào full), meta-bot **size theo độ tin**:

$$
\text{size} = \mathrm{clamp}\!\left(\frac{p - 0.5}{0.5},\ 0,\ 1\right) \cdot \texttt{max\_position\_pct}.
$$

- $p = 0.5$ → size 0 (không cược).
- $p = 0.75$ → size 50% của trần.
- $p = 1.0$ → size 100% của trần.

Về mặt code: thêm một tham số tuỳ chọn `position_pct` vào `Portfolio.buy()` (tương thích ngược — bot thường không truyền thì giữ nguyên hành vi cũ).

### G.3 SL/TP vẫn là lưới cứng

Stop-loss / take-profit vẫn chạy **trước** mọi quyết định của mô hình, đúng như mọi bot khác. Mô hình có thể sai; SL/TP là phanh an toàn không thương lượng.

---

## Phần H. Huấn luyện walk-forward & chống rò rỉ (rủi ro số 1)

### H.1 Vì sao không được trộn ngẫu nhiên (shuffle)?

Trong ML thông thường, ta xáo trộn dữ liệu rồi chia train/test. **Ở đây cấm tuyệt đối.** Lý do: dữ liệu có **trục thời gian**. Nếu train trên dữ liệu *tương lai* rồi test trên *quá khứ*, mô hình "đã biết đáp án" → ảo tưởng giỏi.

### H.2 Walk-forward (tiến theo thời gian)

Ta chia theo **mốc thời gian** $t_{\text{split}}$, không trộn:

```
|—————— train [0, t_split) ——————|—— test [t_split, end) ——|
        (quá khứ, học ở đây)         (tương lai, chấm điểm ở đây)
```

Mô hình chỉ học từ quá khứ, được chấm trên tương lai nó **chưa từng thấy**. Con số trên đoạn test mới đáng tin.

### H.3 Bốn quy tắc chống leakage (bắt buộc)

1. **$w_{t,a}$ as-of $t$:** độ chính xác lịch sử chỉ tính từ dự đoán đã chín **trước** $t$ (Phần D.2).
2. **Regime as-of $t$:** $\sigma_t$, momentum chỉ dùng giá $\le t$.
3. **Nhãn là thông tin tương lai duy nhất được phép:** $y_t$ dùng giá $t+1h$ — đó là *mục tiêu*, đúng theo định nghĩa. Mọi đặc trưng khác phải $\le t$.
4. **Chia theo thời gian, không bao giờ shuffle.**

> Nếu vi phạm bất kỳ quy tắc nào: backtest cho return% rực rỡ, chạy thật lỗ. **Mọi khẳng định "meta thắng" chỉ có giá trị nếu được đo trên đoạn test walk-forward sạch.**

### H.4 Quy trình `train_meta_for_market(market, symbol=None)`

```
1. Gom các hàng prediction căn theo (mã, timestamp):
     với mỗi (mã, t):  g_{t,a} từ *_predictions
                        w_{t,a} tính as-of t (chỉ target chín trước t)
                        sigma_t, mom_t từ giá ≤ t
                        nhãn y_t từ giá thật t+1h
2. Sắp theo thời gian; cắt train [0, t_split) / test [t_split, end).
3. Train LightGBM trên train; hiệu chỉnh isotonic trên slice held-out.
4. Báo cáo trên test: direction accuracy out-of-sample, Brier score, calibration curve.
5. Lưu checkpoint:  meta_{market}.pkl              (pooled: gộp mọi mã)
                    meta_{market}_{symbol}.pkl     (per-symbol: từng mã, guard PER_SYMBOL_MIN_POINTS)
```

Huấn luyện qua cron `train_meta` (Chủ nhật, sau các train khác) hoặc trigger tay.

---

## Phần I. Dự phòng (fallback) — không bao giờ vỡ pipeline

Thiếu LightGBM, thiếu checkpoint, hay lỗi nạp model → rơi về **reliability-weighted vote** (bỏ phiếu có trọng số), một heuristic minh bạch không cần train:

$$
p_{\text{fallback}} = \frac{1}{2} + \frac{1}{2}\cdot\frac{\sum_{a} \tilde w_a \cdot \mathrm{sign}(g_{t,a})}{\sum_{a} |\tilde w_a|},
\qquad
\tilde w_a = \big(w_{t,a} - 0.5\big).
$$

Giải nghĩa:

- $\mathrm{sign}(g_{t,a})$ = thuật toán $a$ bỏ phiếu lên (+1) hay xuống (−1).
- $\tilde w_a = w_{t,a} - 0.5$ là **trọng số có dấu**: thuật toán đúng >50% có trọng số dương (nghe theo); thuật toán **<0.45 có trọng số âm → phiếu của nó bị ĐẢO DẤU** (làm ngược lời). Đây chính là cách khai thác GOLD lightgbm-25%.
- Kết quả ép về $[0,1]$ quanh 0.5.

Pipeline nhờ vậy luôn ra được một $p$ để giao dịch, dù mô hình chính chưa sẵn sàng.

---

## Phần J. Hai biến thể bot

| Biến thể | Key | Số bot | Checkpoint | Khi nào |
|---|---|---|---|---|
| **Pooled** | `meta_stack` | 4 (1/thị trường) | `meta_{market}.pkl` | Luôn bật |
| **Per-symbol** | `meta_stack__ps` | = số mã/thị trường | `meta_{market}_{symbol}.pkl` | `PER_SYMBOL_ENABLED=true` |

- **Pooled** gộp mọi mã của một thị trường vào **một** mô hình → nhiều dữ liệu, vững nhưng "trung bình hoá".
- **Per-symbol** mỗi mã một mô hình → cá nhân hoá nhưng **đói dữ liệu** (ít điểm/mã).

Cả hai chạy chung leaderboard, **vốn khởi đầu $1000** — để chính số liệu thực tế trả lời "pooled hay per-symbol thắng".

---

## Phần K. "Xem con số" — cách so trực tiếp

Train xong → gọi `POST /api/trigger/simulation-backtest` → leaderboard hiện ngay **return% / Sharpe / win-rate / max drawdown** của `meta_stack` (và `meta_stack__ps`) **cạnh** 11 bot per-algo, cùng mốc $1000. Đây là phép so trực tiếp meta-stacking vs từng thuật toán đơn lẻ.

Hai thước đo cần nhìn (đừng chỉ nhìn return thô):

$$
\text{Sharpe} = \frac{\mathbb{E}[R]}{\sqrt{\operatorname{Var}[R]}}\sqrt{P}, \qquad
\text{MaxDD} = \max_t\Big(1 - \frac{V_t}{\max_{\tau\le t} V_\tau}\Big).
$$

(Sharpe = lợi nhuận trên rủi ro, càng cao càng tốt; MaxDD = sụt giảm sâu nhất, càng nhỏ càng tốt.)

---

## Phần L. Điểm mạnh / yếu

**Mạnh**
- **Hội ý toàn bộ** thuật toán thay vì tin mù một cái.
- **Biết ai đáng tin** qua $w_{t,a}$, và **đảo dấu** thuật toán sai có hệ thống.
- **Size theo độ chắc** thay vì luôn vào full.
- **Giải thích được**: LightGBM cho feature importance → biết bot đang tin thuật toán nào.
- **Không bao giờ vỡ**: fallback vote khi thiếu model.

**Yếu / giới hạn**
- **Biên lợi thế nhỏ**: nhiều thuật toán ~50% → meta nhỉnh hơn nhưng phí giao dịch theo giờ có thể nuốt phần lợi. Phải đo PnL **ròng**.
- **Per-symbol đói data** → dễ thua pooled.
- **Leakage là tử huyệt**: chỉ một lỗi as-of là toàn bộ con số thành vô nghĩa.
- **Phụ thuộc chất lượng `algos/`**: rác vào → rác ra. Meta không tạo ra tín hiệu mới, nó chỉ tổng hợp tín hiệu đã có.

---

## Phần M. Bảng tham số

| Nhóm | Tham số | Giá trị mặc định |
|---|---|---|
| Đặc trưng | $K$ (cửa sổ rolling accuracy) | 40 |
| Đặc trưng | cửa sổ volatility $n$ | 20 |
| Đặc trưng | chiều $x_t$ | $2A + 2$ |
| Model | LightGBM | gradient boosting nhị phân, `class_weight="balanced"` |
| Model | calibration | sigmoid (Platt); bỏ qua nếu calib < 30 mẫu hoặc p_std < 0.02 |
| Chính sách | margin $\delta$ | `buy_threshold`/100 |
| Chính sách | ngưỡng thích ứng | `max(0.5+δ, μ_p + K·σ_p)`, K=1.0; per-symbol → `0.5+δ` |
| Chính sách | sizing | conviction (Phần G.2) |
| Fallback | ngưỡng đảo dấu | $w_a < 0.45$ |
| Vốn | initial capital | $1000 |
| Per-symbol | guard tối thiểu | `PER_SYMBOL_MIN_POINTS` |

---

## Phần N. Phụ thuộc

- **LightGBM** — có trong `[ml]` extras; thiếu → fallback reliability-weighted vote.
- **scikit-learn** (isotonic calibration) — thiếu → bỏ hiệu chỉnh, dùng xác suất thô.
- **numpy**.
- Dữ liệu `*_predictions` với cột `direction_correct` (đã có).
- Liên quan: [meta-rl.md](meta-rl.md) (biến thể Reinforcement Learning), `algos/12_rldqn.md` (mẫu format), `algos/features.md`.
