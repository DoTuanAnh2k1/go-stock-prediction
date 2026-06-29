# Meta-RL — Reinforcement Learning đọc dự đoán của các thuật toán (giải thích từ số 0)

> **⚠️ Đây là THIẾT KẾ TƯƠNG LAI, CHƯA TRIỂN KHAI.** Tài liệu này đặc tả phần toán của một biến thể Reinforcement Learning cho chiến thuật [Meta-Stacking](meta-stacking.md). Bản đang được code là bản supervised (LightGBM). Meta-RL ở đây để chốt công thức trước, code sau.

**Loại:** Chiến thuật giao dịch (trading tactic) — KHÔNG phải thuật toán dự đoán.
**Key bot (dự kiến):** `meta_rl` / `meta_rl__ps`
**Quan hệ:** cùng mục tiêu với [meta-stacking.md](meta-stacking.md) (đọc tất cả dự đoán rồi quyết định), nhưng học bằng RL thay vì học có giám sát.
**Mẫu format:** `algos/12_rldqn.md` — Meta-RL **giống hệt** rl_dqn về khung toán (MDP / Bellman / DQN / Double DQN / replay / ε-greedy / reward), **chỉ khác trạng thái** (nhồi thêm vector dự đoán).

> Tài liệu viết cho người chưa biết RL. Chỉ cần toán cao cấp cơ bản: tổng $\sum$, hàm số, gradient, vector & ma trận, "trung bình". Mỗi thuật ngữ định nghĩa trước, công thức sau.

---

## Phần A. Trực giác: khác Meta-Stacking ở đâu?

[Meta-Stacking](meta-stacking.md) thuê một **trưởng nhóm học có giám sát**: ta đưa cho anh ta hàng nghìn ví dụ "tình huống → giá đã lên hay xuống", anh ta học đoán xác suất lên. Anh ta tối ưu **đoán đúng hướng**.

Meta-RL thuê một **trưởng nhóm học bằng thử–sai**. Không ai bảo anh ta "giờ này phải mua". Anh ta tự ra quyết định mua/bán/giữ, cuối mỗi bước nhận **lãi/lỗ** làm phản hồi, và dần học **chuỗi hành động nào làm tổng tiền lãi lâu dài lớn nhất**. Anh ta tối ưu **ra quyết định tốt** (tiền), không phải đoán đúng số.

Điểm chung với rl_dqn (`algos/12_rldqn.md`): cùng là Deep Q-Network. **Điểm khác duy nhất nhưng cốt lõi:** rl_dqn chỉ nhìn **đặc trưng kỹ thuật** của giá; Meta-RL nhìn thêm **dự đoán của tất cả thuật toán `algos/` + độ tin cậy lịch sử của chúng**. Nói cách khác, Meta-RL là rl_dqn **được cho xem bài của 11 nhà phân tích** trước khi quyết.

---

## Phần B. Khung MDP (Markov Decision Process)

RL chuẩn hoá bài toán thành bộ năm phần tử:

$$
\mathcal{M} = \langle\, \mathcal{S},\ \mathcal{A},\ P,\ R,\ \gamma \,\rangle.
$$

- $\mathcal{S}$ — tập trạng thái (Phần C — **đây là chỗ Meta-RL khác rl_dqn**).
- $\mathcal{A} = \{0:\text{hold},\,1:\text{buy},\,2:\text{sell}\}$ — ba hành động.
- $P(s'\mid s,a)$ — luật chuyển trạng thái (do thị trường + động học vị thế quyết định).
- $R(s,a)$ — phần thưởng kỳ vọng (Phần E).
- $\gamma = 0.99$ — hệ số chiết khấu (coi trọng tương lai nhưng có giới hạn).

**"Markov"** = tương lai chỉ phụ thuộc hiện tại. Ta ép giả định này đúng bằng cách **nhét trạng thái vị thế vào $s$** (Phần C.4), nhờ đó "hiện tại" chứa đủ thông tin để quyết.

---

## Phần C. Trạng thái $s_t$ — điểm khác biệt cốt lõi

### C.1 Công thức tổng

$$
s_t = \big[\,\phi_t \ \big\Vert\ g_t \ \big\Vert\ w_t \ \big\Vert\ (\rho_t,\ u_t,\ h_t)\,\big].
$$

Ký hiệu $\Vert$ = ghép nối vector. Bốn khối:

### C.2 $\phi_t$ — đặc trưng kỹ thuật (giống rl_dqn)

$\phi_t \in \mathbb{R}^{30}$ — 30 đặc trưng kỹ thuật của thị trường (lag returns, MA ratios, RSI, StochRSI, Bollinger %B, MACD, volatility, ROC, momentum, volume ratio). Xem `algos/features.md`. Đây là toàn bộ những gì rl_dqn nhìn thấy.

### C.3 $g_t$ và $w_t$ — phần MỚI (dự đoán của các thuật toán)

$$
g_t = \big(g_{t,1},\dots,g_{t,A}\big), \qquad g_{t,a} = \frac{\hat p_{t,a} - p_t}{p_t},
$$

$$
w_t = \big(w_{t,1},\dots,w_{t,A}\big), \qquad w_{t,a} = \text{direction accuracy rolling K=40 của algo } a, \text{ as-of } t.
$$

- $g_{t,a}$ = % thay đổi mà thuật toán $a$ dự đoán (ý kiến của nhà phân tích $a$).
- $w_{t,a}$ = nhà phân tích $a$ gần đây đáng tin tới đâu (xem [meta-stacking.md §D.2](meta-stacking.md) cho công thức đầy đủ + cảnh báo leakage).

Đây chính là khối khiến agent **đọc được bài của các thuật toán** thay vì chỉ nhìn giá thô.

### C.4 $(\rho_t, u_t, h_t)$ — trạng thái vị thế (giống rl_dqn)

- $\rho_t \in \{0,1\}$ — đang giữ long (1) hay flat (0).
- $u_t = \dfrac{p_t}{p_{\text{entry}}} - 1$ — lãi/lỗ chưa chốt (0 nếu flat).
- $h_t = \min\!\big(\tfrac{\text{số bước giữ}}{30}, 1\big)$ — thời gian giữ, chuẩn hoá $[0,1]$.

### C.5 Chiều của trạng thái

$$
\dim(s_t) = \underbrace{30}_{\phi_t} + \underbrace{A}_{g_t} + \underbrace{A}_{w_t} + \underbrace{3}_{\text{vị thế}} = 33 + 2A.
$$

So với rl_dqn (33 chiều), Meta-RL **tăng thêm $2A$ chiều** — đúng bằng hai khối dự đoán + độ tin. Với $A=11$ thuật toán → $\dim(s_t) = 55$. Đây là **toàn bộ** thay đổi về cấu trúc; mọi phần còn lại y hệt rl_dqn.

---

## Phần D. Mục tiêu & giá trị (giống rl_dqn)

### D.1 Return — tổng phần thưởng chiết khấu

$$
G_t = \sum_{k=0}^{\infty} \gamma^k\, r_{t+k+1}.
$$

$\gamma^k$ làm phần thưởng càng xa càng nhẹ ký; $\gamma<1$ giúp tổng hội tụ.

### D.2 Hàm giá trị $Q$

$$
Q^\pi(s,a) = \mathbb{E}_\pi\big[\,G_t \mid s_t=s,\ a_t=a\,\big].
$$

$Q$ ("quality") = "ở tình huống này, nước đi nào đáng giá nhất về lâu dài".

### D.3 Bellman & chính sách tối ưu

$$
Q^*(s,a) = \mathbb{E}\!\Big[\,r_{t+1} + \gamma\max_{a'}Q^*(s_{t+1},a')\ \Big|\ s_t=s,\ a_t=a\,\Big],
\qquad
\pi^*(s) = \arg\max_a Q^*(s,a).
$$

Học RL ở đây = học xấp xỉ hàm $Q^*$.

---

## Phần E. Phần thưởng (giống rl_dqn — tối ưu tiền, không tối ưu đoán đúng)

$$
r_{t+1} = \underbrace{\rho_t\!\left(\frac{p_{t+1}}{p_t}-1\right)}_{\text{lãi/lỗ theo vị thế}}
\ -\ \underbrace{c\,\big|\rho_t - \rho_{t-1}\big|}_{\text{phí khi vào/ra lệnh}}
\ -\ \underbrace{\lambda\,\rho_t}_{\text{phạt ôm long}}.
$$

- $c = 0.0005$ — phí, chỉ trừ khi đổi trạng thái.
- $\lambda = 0.0001$ — phạt nhỏ mỗi bước đang long, để agent không ôm long vĩnh viễn.

Lưu ý: reward là **tiền thật**, nên agent có thể học chiến lược mà direction accuracy chưa chắc cao nhưng PnL tốt (ví dụ ít giao dịch, vào đúng lúc lớn).

---

## Phần F. DQN — mạng Q và các mẹo giữ ổn định (giống rl_dqn)

### F.1 Mạng Q (MLP 3 lớp) — chỉ in_dim đổi

$$
\begin{aligned}
h^{(1)} &= \mathrm{ReLU}(W_1 s + b_1), & W_1 &\in \mathbb{R}^{128 \times (33+2A)},\\
h^{(2)} &= \mathrm{ReLU}(W_2 h^{(1)} + b_2), & W_2 &\in \mathbb{R}^{64 \times 128},\\
Q_\theta(s,\cdot) &= W_3 h^{(2)} + b_3, & W_3 &\in \mathbb{R}^{3 \times 64}.
\end{aligned}
$$

Chỉ **lớp vào** rộng ra ($33 \to 33+2A$) để nuốt thêm $g_t, w_t$; phần còn lại của mạng giữ nguyên kích thước.

### F.2 TD error & loss Huber

Mục tiêu một bước và loss:

$$
y = r + \gamma(1-d)\max_{a'} Q_{\theta^-}(s',a'), \qquad
\mathcal{L}(\theta) = \mathbb{E}_{(s,a,r,s',d)\sim\mathcal{D}}\big[\ell_\delta\big(y - Q_\theta(s,a)\big)\big],
$$

với $\ell_\delta$ = Huber loss (bình phương khi sai số nhỏ, tuyến tính khi lớn → bền với nhiễu), $\delta=1$:

$$
\ell_\delta(x) =
\begin{cases}
\tfrac12 x^2, & |x|\le\delta,\\
\delta(|x| - \tfrac12\delta), & |x|>\delta.
\end{cases}
$$

### F.3 Ba mẹo ổn định (như rl_dqn)

- **Replay buffer** $\mathcal{D}$: lưu $(s,a,r,s',d)$, mỗi lần học bốc ngẫu nhiên minibatch → phá tương quan thời gian. Sức chứa 10 000, batch 64, bắt đầu học khi $\ge 200$ trải nghiệm.
- **Target network** $\theta^-$: bản sao đông cứng để tính mục tiêu, đồng bộ mỗi $C=50$ bước → mục tiêu đứng yên đủ lâu.
- **Double DQN** (chống lạc quan ảo): mạng chính chọn hành động, mạng mục tiêu chấm điểm:

$$
a^* = \arg\max_{a'} Q_\theta(s',a'), \qquad
y = r + \gamma(1-d)\,Q_{\theta^-}(s',a^*).
$$

### F.4 Bước cập nhật (Adam + clip gradient)

$$
\theta \leftarrow \theta - \eta\,\widehat{\nabla_\theta\mathcal{L}}, \qquad
\|\nabla_\theta\mathcal{L}\|_2 \le 1, \qquad \eta = 10^{-3}.
$$

---

## Phần G. Khám phá vs khai thác (ε-greedy, giống rl_dqn)

Với xác suất $\epsilon$ chọn ngẫu nhiên, còn lại chọn greedy ($\arg\max_a Q_\theta$). Đầu train $\epsilon$ cao (khám phá), giảm tuyến tính về thấp; lúc dùng thật đặt $\epsilon=0$:

$$
\epsilon(g) = \max\!\Big(\epsilon_{\min},\ \epsilon_{\max} - (\epsilon_{\max}-\epsilon_{\min})\tfrac{g}{G}\Big),
\quad \epsilon_{\max}=1.0,\ \epsilon_{\min}=0.05,
$$

với $g$ = số bước đã đi, horizon $G = 0.6\cdot E\cdot\sum_i (L_i - 1)$ ($E$ = số epoch, $L_i$ = độ dài episode) — tính theo dữ liệu thực để thị trường ít dữ liệu vẫn kịp giảm $\epsilon$.

---

## Phần H. Pseudocode huấn luyện (Double DQN)

```
Khởi tạo policy θ, target θ⁻ ← θ, buffer D rỗng, đếm bước g = 0
Tính horizon ε:  G = 0.6 · E · Σ_i (L_i − 1)

lặp E epoch:
    xáo trộn các episode (chuỗi thật + chuỗi "gương" mirror, đã cắt cửa sổ)
    với mỗi episode:
        ρ ← 0 (flat)
        với mỗi bước t:
            s   ← [ φ_t ‖ g_t ‖ w_t ‖ (ρ, lãi chưa chốt, thời gian giữ) ]   # ← khác rl_dqn: có g_t, w_t
            ε   ← ε(g);  g ← g + 1
            a   ← ngẫu nhiên nếu rand < ε, ngược lại argmax_a Q_θ(s,a)
            cập nhật ρ theo a;  tính thưởng r (Phần E)
            s'  ← trạng thái kế tiếp
            lưu (s, a, r, s', done) vào D
            nếu |D| ≥ 200: lặp ≤ 10 lần:
                bốc 64 mẫu từ D
                y ← mục tiêu Double DQN
                hạ loss Huber bằng Adam (clip ‖∇‖ ≤ 1)
                mỗi 50 bước: θ⁻ ← θ
    log: ε, loss trung bình, tổng thưởng
Đánh giá greedy (ε=0) trên chuỗi thật → greedy_total_reward
Lưu checkpoint meta_rl_{market}.pt
```

> **Lưu ý leakage khi dựng dữ liệu train:** $g_t, w_t$ phải as-of $t$ — $w_t$ chỉ dùng dự đoán đã chín trước $t$, $g_t$ là dự đoán phát ra tại $t$. Sai một li là agent nhìn trộm tương lai (xem [meta-stacking.md §H](meta-stacking.md)).

---

## Phần I. Hai vai trò khi dùng

### I.1 Vai trò bot giao dịch native (chính)

Ở tầng simulation, bot `{market}_meta_rl` để policy tự quyết, không dùng luật ngưỡng:

```
SL/TP safety check (lưới an toàn cứng)
với mỗi mã của thị trường:
    s = [ φ(giá ≤ ngày sim) ‖ g (dự đoán algo) ‖ w (accuracy as-of) ‖ vị thế THẬT của bot ]
    a = argmax_a Q_θ(s)      (greedy, ε = 0)
    thực thi BUY/SELL/HOLD trực tiếp
```

Backtest cần giá "as-of" (`get_*_prices_asc_as_of`) **và** dự đoán as-of để dựng trạng thái đúng thời điểm quá khứ.

### I.2 Độ tin cậy (nếu cần xuất confidence)

Như rl_dqn, dùng softmax trên Q:

$$
\text{confidence} = \big[\mathrm{softmax}\,Q_\theta(s)\big]_a = \frac{e^{Q_\theta(s)_a}}{\sum_{a'} e^{Q_\theta(s)_{a'}}}.
$$

---

## Phần J. Bảng hyperparameter (kế thừa rl_dqn, chỉ in_dim đổi)

| Nhóm | Tham số | Giá trị |
|---|---|---|
| Mạng | HIDDEN1 / HIDDEN2 / N_ACTIONS | 128 / 64 / 3 |
| Mạng | **in_dim (30 features + 2A dự đoán + 3 vị thế)** | **33 + 2A** (≈55 với A=11) |
| RL | GAMMA (γ) | 0.99 |
| RL | LR_DQN (η, Adam) | 0.001 |
| RL | REPLAY_CAPACITY | 10 000 |
| RL | BATCH_SIZE | 64 |
| RL | MIN_REPLAY | 200 |
| RL | TARGET_UPDATE_EVERY (C) | 50 bước gradient |
| RL | EPSILON_START / END | 1.0 / 0.05 |
| RL | TRAIN_EPOCHS (E) | 12 |
| Reward | transaction_cost (c) | 0.0005 |
| Reward | HOLD_LONG_PENALTY (λ) | 0.0001 |
| Augment | EPISODE_WINDOW (W) / STRIDE (S) | 130 / 35 |
| Đặc trưng | K (rolling accuracy của w_t) | 40 |
| Loss | Huber δ / clip-norm | 1.0 / 1.0 |

---

## Phần K. So sánh Meta-RL vs Meta-Stacking vs rl_dqn

| | rl_dqn | Meta-Stacking | Meta-RL |
|---|---|---|---|
| Học kiểu | RL (DQN) | Supervised (LightGBM) | RL (DQN) |
| Nhìn thấy dự đoán algo? | **Không** | **Có** ($g, w$) | **Có** ($g, w$) |
| Tối ưu | PnL | Đoán đúng hướng (P up) | PnL |
| Đầu ra | hành động | xác suất → hành động | hành động |
| Trạng thái | 33 chiều | (không phải RL) | **33 + 2A chiều** |
| Triển khai | Đã có | Đang code | **Tương lai** |

---

## Phần L. Điểm mạnh / yếu (so với Meta-Stacking)

**Mạnh**
- Tối ưu thẳng **tiền lãi lâu dài**, không chỉ đoán đúng một bước.
- Trạng thái chứa vị thế ⇒ học được "đang giữ thì nên thoát khi nào".
- Nhìn cả dự đoán algo ⇒ thông minh hơn rl_dqn thuần.

**Yếu / giới hạn**
- **Ngốn dữ liệu hơn** Meta-Stacking; biên lợi thế nhỏ + ít dữ liệu ⇒ khó hội tụ ổn định.
- **Khó giải thích** "vì sao mua" hơn LightGBM (vốn có feature importance).
- **Nhạy hyperparameter**; phải xác nhận qua PnL out-of-sample theo thời gian, không tin mỗi reward train.
- Vì các lý do trên, **bản supervised được làm trước**; Meta-RL chỉ nên làm khi Meta-Stacking đã chứng minh có biên lợi thế thật.

---

## Phần M. Phụ thuộc

- `torch` (PyTorch) — bắt buộc; thiếu thì không chạy được Meta-RL.
- `numpy`.
- Dữ liệu `*_predictions` (cho $g_t$) + `direction_correct` (cho $w_t$).
- `build_enhanced_features` (cho $\phi_t$ — xem `algos/features.md`).
- Liên quan: [meta-stacking.md](meta-stacking.md) (bản supervised đang triển khai), `algos/12_rldqn.md` (khung RL gốc).
