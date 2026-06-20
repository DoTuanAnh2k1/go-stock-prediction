# RL DQN — Deep Q-Network Reinforcement Learning (giải thích từ số 0)

**Key:** `rl_dqn`
**Class:** `RLDQNPredictor`
**File:** [prediction-svc/src/algorithms/rl_dqn.py](../prediction-svc/src/algorithms/rl_dqn.py)
**Stateful:** Có — `train_batch()` huấn luyện DQN và **lưu checkpoint xuống đĩa** (`${RL_MODEL_DIR}/rl_dqn_{market}.pt`)
**Vai trò kép:** (1) thuật toán dự đoán thứ 12; (2) trading bot **native** ở tầng simulation

> **Tài liệu này viết cho người chưa biết gì về Reinforcement Learning.** Chỉ cần biết toán cao cấp cơ bản: tổng $\sum$, hàm số, đạo hàm/gradient, vector & ma trận, và khái niệm "trung bình". Mỗi thuật ngữ được **định nghĩa và giải thích trực giác trước**, công thức đến sau. Công thức render trên GitHub và bản xem trước Markdown của VSCode.

---

## Phần A. Trực giác: bài toán là gì?

Hãy tưởng tượng bạn ngồi ở một bàn giao dịch. Mỗi ngày bạn nhìn bảng giá rồi quyết định **một trong ba việc**: *mua* (vào lệnh), *bán* (thoát lệnh), hoặc *không làm gì* (giữ nguyên). Cuối ngày, túi tiền của bạn lãi hoặc lỗ một chút tuỳ quyết định vừa rồi và việc giá hôm sau lên hay xuống.

Bạn **không có thầy** dạy "ngày này phải mua, ngày kia phải bán". Bạn chỉ có **kết quả** (lãi/lỗ) sau mỗi quyết định. Học bằng cách **thử – sai – rút kinh nghiệm** như vậy chính là **Reinforcement Learning (Học tăng cường, RL)**.

So sánh với 11 thuật toán còn lại:

- 11 thuật toán kia là **học có giám sát**: có "đáp án đúng" là *giá thật ngày mai*, và chúng cố đoán con số đó cho gần nhất. Mục tiêu là **đoán đúng con số**.
- `rl_dqn` thì **không đoán con số**. Nó học **nên hành động thế nào** để **tổng tiền lãi về lâu dài** lớn nhất. Mục tiêu là **ra quyết định tốt**.

Phần còn lại của tài liệu sẽ biến câu chuyện bàn giao dịch này thành toán, từng bước một.

---

## Phần B. Các khái niệm nền tảng

RL có một bộ từ vựng. Ta định nghĩa lần lượt, kèm "trong dự án này nó là gì".

### B.1 Agent (tác nhân) và Environment (môi trường)

- **Agent** = người/chương trình ra quyết định. Ở đây là **mạng nơ-ron** của ta.
- **Environment** = thế giới mà agent tương tác. Ở đây là **thị trường** (chuỗi giá).

Vòng lặp tương tác: agent nhìn môi trường → chọn một hành động → môi trường thay đổi và trả về phần thưởng → lặp lại.

```
        ┌─────────────────────────────────────────┐
        │                                         │
        ▼                                         │
   [ Environment ] ──(trạng thái s, phần thưởng r)──► [ Agent ]
        ▲                                         │
        │                (hành động a)            │
        └─────────────────────────────────────────┘
```

### B.2 State (trạng thái) $s$

> **Định nghĩa.** Trạng thái là "tất cả những gì agent quan sát được tại một thời điểm để ra quyết định".

Trong dự án: $s$ gồm các con số mô tả thị trường hôm nay (xu hướng, độ biến động, RSI, MACD…) **cộng thêm** tình trạng túi tiền của ta (đang giữ lệnh hay không). Chi tiết ở Phần H.

### B.3 Action (hành động) $a$

> **Định nghĩa.** Hành động là lựa chọn mà agent đưa ra ở mỗi bước.

Trong dự án: $a$ thuộc tập 3 lựa chọn $\{\text{hold},\ \text{buy},\ \text{sell}\}$.

### B.4 Reward (phần thưởng) $r$

> **Định nghĩa.** Phần thưởng là con số môi trường trả về sau mỗi hành động, đo "việc vừa làm tốt hay tệ". Agent muốn **tổng phần thưởng** càng lớn càng tốt.

Trong dự án: phần thưởng ≈ **tiền lãi/lỗ** của bước đó (trừ phí). Lãi → thưởng dương; lỗ → thưởng âm.

### B.5 Policy (chính sách) $\pi$

> **Định nghĩa.** Chính sách là "chiến lược" của agent: một hàm nhận trạng thái và trả về hành động (hoặc xác suất các hành động).

Ký hiệu $\pi(a\mid s)$ = xác suất chọn hành động $a$ khi đang ở trạng thái $s$. **Học RL = tìm chính sách tốt.**

### B.6 Episode & Trajectory (tập & quỹ đạo)

- **Episode** = một "ván chơi" đầy đủ, ví dụ chạy qua hết một đoạn chuỗi giá từ đầu đến cuối.
- **Trajectory** $\tau$ = chuỗi những gì xảy ra trong một episode:

$$
\tau=\big(s_0,\,a_0,\,r_1,\,s_1,\,a_1,\,r_2,\,s_2,\dots\big).
$$

Đọc là: ở trạng thái $s_0$ làm $a_0$, nhận thưởng $r_1$ và sang $s_1$; rồi tiếp tục.

### B.7 MDP — gói tất cả lại

Toàn bộ thiết lập trên gọi là **Markov Decision Process (Quá trình quyết định Markov, MDP)** — khung toán chuẩn của RL. Nó là một bộ năm phần tử:

$$
\mathcal{M}=\langle\,\mathcal{S},\ \mathcal{A},\ P,\ R,\ \gamma\,\rangle.
$$

Trong đó:

- $\mathcal{S}$ — **tập mọi trạng thái** có thể.
- $\mathcal{A}$ — **tập mọi hành động** có thể.
- $P(s'\mid s,a)$ — **luật chuyển trạng thái**: xác suất sang $s'$ nếu ở $s$ làm $a$.
- $R(s,a)$ — **phần thưởng kỳ vọng** khi ở $s$ làm $a$.
- $\gamma\in[0,1)$ — **hệ số chiết khấu** (giải thích ở C.2). Ở đây $\gamma=0.99$.

> **Chữ "Markov" nghĩa là gì?** Là giả định: *tương lai chỉ phụ thuộc hiện tại, không phụ thuộc quá khứ*. Tức biết trạng thái hôm nay là đủ, không cần nhớ toàn bộ lịch sử:
>
> $$
> P\big(s_{t+1}\mid s_t,a_t,\ s_{t-1},a_{t-1},\dots\big)=P\big(s_{t+1}\mid s_t,a_t\big).
> $$
>
> Ta làm cho giả định này đúng bằng cách **nhét tình trạng vị thế vào trạng thái** (Phần H), nhờ đó "hôm nay" đã chứa đủ thông tin để quyết định.

---

## Phần C. Biến mục tiêu thành toán

### C.1 Return — tổng phần thưởng

Agent không tham lam phần thưởng *một bước*; nó muốn **tổng phần thưởng về sau**. Tổng đó (tính từ bước $t$) gọi là **return** $G_t$:

$$
G_t=r_{t+1}+\gamma\, r_{t+2}+\gamma^{2} r_{t+3}+\cdots=\sum_{k=0}^{\infty}\gamma^{k}\,r_{t+k+1}.
$$

Trong đó:

- $r_{t+1},r_{t+2},\dots$ — phần thưởng các bước tương lai.
- $\gamma^k$ — trọng số giảm dần cho phần thưởng càng xa.

### C.2 Vì sao có $\gamma$ (chiết khấu)?

$\gamma<1$ làm phần thưởng tương lai "nhẹ ký" hơn phần thưởng gần. Hai lý do:

1. **Trực giác tài chính:** 1 đồng hôm nay quý hơn 1 đồng năm sau.
2. **Toán học:** nếu chuỗi vô hạn, $\gamma<1$ giúp tổng **hội tụ** (không thành vô cực).

Với $\gamma=0.99$, phần thưởng sau $k$ bước bị nhân $0.99^{k}$ — vẫn coi trọng tương lai nhưng có giới hạn.

### C.3 "Kỳ vọng" $\mathbb{E}$ — chỉ là trung bình

Vì thị trường có ngẫu nhiên, cùng một chính sách có thể cho return khác nhau mỗi lần chạy. Ta quan tâm **giá trị trung bình** của return, ký hiệu $\mathbb{E}[\cdot]$ (đọc là "kỳ vọng", nghĩa nôm na là "trung bình qua mọi khả năng").

### C.4 Mục tiêu học

Tìm chính sách $\pi$ sao cho **return trung bình** lớn nhất:

$$
\max_{\pi}\ J(\pi),\qquad J(\pi)=\mathbb{E}_{\tau\sim\pi}\!\left[\sum_{t\ge 0}\gamma^{t}\,r_{t+1}\right].
$$

Đọc: lấy trung bình (qua các quỹ đạo $\tau$ sinh ra bởi $\pi$) của tổng phần thưởng chiết khấu.

---

## Phần D. "Giá trị" và phương trình Bellman

Làm sao biết một nước đi là tốt nếu phần thưởng thật sự chỉ lộ ra ở tương lai? Ta cần một thước đo "độ tốt về lâu dài". Đó là **hàm giá trị**.

### D.1 Hai hàm giá trị

> **Định nghĩa.** $V^\pi(s)$ = return trung bình nếu **bắt đầu ở $s$** rồi đi theo $\pi$. $Q^\pi(s,a)$ = return trung bình nếu **ở $s$, làm $a$ trước**, rồi mới theo $\pi$.

$$
V^{\pi}(s)=\mathbb{E}_{\pi}\!\big[\,G_t\mid s_t=s\,\big],\qquad
Q^{\pi}(s,a)=\mathbb{E}_{\pi}\!\big[\,G_t\mid s_t=s,\ a_t=a\,\big].
$$

$Q$ ("quality") trả lời đúng câu ta cần: **"ở tình huống này, nước đi nào đáng giá nhất?"**

### D.2 Phương trình Bellman — ý tưởng đệ quy

Mấu chốt: "giá trị bây giờ = phần thưởng ngay + giá trị của trạng thái kế tiếp (đã chiết khấu)". Viết bằng lời rồi bằng công thức:

> Giá trị của (làm $a$ ở $s$) = thưởng nhận ngay $+\ \gamma\times$ giá trị trung bình của bước tiếp theo.

$$
Q^{\pi}(s,a)=\mathbb{E}\!\big[\,r_{t+1}+\gamma\,Q^{\pi}(s_{t+1},a_{t+1})\ \big|\ s_t=s,\ a_t=a\,\big].
$$

Đây gọi là **phương trình Bellman**. Nó cho phép tính giá trị **mà không cần** mô phỏng vô hạn về tương lai — chỉ cần nhìn một bước rồi dựa vào giá trị bước sau.

### D.3 Chính sách tối ưu

Nếu biết được hàm $Q$ **tốt nhất có thể** (ký hiệu $Q^{*}$), thì chiến lược tối ưu chỉ là "mỗi bước chọn hành động có $Q^{*}$ lớn nhất":

$$
Q^{*}(s,a)=\mathbb{E}\!\Big[\,r_{t+1}+\gamma\,\max_{a'}Q^{*}(s_{t+1},a')\ \Big|\ s_t=s,\ a_t=a\,\Big],
\qquad
\pi^{*}(s)=\arg\max_{a}\,Q^{*}(s,a).
$$

Ký hiệu $\arg\max_a$ nghĩa là "chọn hành động $a$ làm cho biểu thức lớn nhất". **Vậy: học RL ở đây quy về học hàm $Q^{*}$.**

---

## Phần E. Từ bảng tra cứu đến mạng nơ-ron (DQN)

### E.1 Vì sao không dùng bảng?

Nếu số trạng thái ít, ta có thể lập **bảng** $Q$ (mỗi ô = một cặp trạng thái–hành động) và cập nhật dần. Nhưng trạng thái của ta là **vector 33 số thực liên tục** → vô hạn ô → không lập bảng được.

### E.2 Mạng nơ-ron = "máy xấp xỉ hàm"

Ta thay bảng bằng một **mạng nơ-ron** $Q_\theta$ với bộ tham số $\theta$ (các trọng số). Hiểu đơn giản: $Q_\theta$ là một **hàm có thể điều chỉnh được**, nhận vào trạng thái $s$ và in ra 3 con số — giá trị ước lượng của hold/buy/sell. "Huấn luyện" = chỉnh $\theta$ để 3 con số đó ngày càng đúng. Phương pháp này gọi là **Deep Q-Network (DQN)**.

### E.3 Học bằng cách giảm "độ bất ngờ" (TD error)

Phương trình Bellman nói: $Q(s,a)$ *nên bằng* $r+\gamma\max_{a'}Q(s',a')$. Nếu mạng đang dự đoán lệch, độ lệch đó gọi là **TD error** (sai số temporal-difference) — chính là "độ bất ngờ":

$$
\delta=\underbrace{\big(r+\gamma\max_{a'}Q(s',a')\big)}_{\text{nên bằng (mục tiêu)}}-\underbrace{Q(s,a)}_{\text{đang đoán}}.
$$

Học = chỉnh $\theta$ để $\delta$ nhỏ lại. Đây là ý tưởng của Q-learning; dạng bảng cập nhật:

$$
Q(s_t,a_t)\leftarrow Q(s_t,a_t)+\alpha\,\delta_t,
$$

với $\alpha$ = tốc độ học (chỉnh mạnh hay nhẹ).

---

## Phần F. DQN học thế nào (và các "mẹo" giữ ổn định)

### F.1 Hàm mất mát (loss)

Với mạng nơ-ron, ta định nghĩa **loss** = trung bình bình phương (hoặc Huber) của TD error, rồi dùng **gradient descent** để giảm nó.

> **Gradient descent là gì?** Tưởng tượng loss là một thung lũng theo các tham số $\theta$. Gradient $\nabla_\theta$ chỉ hướng dốc lên; ta bước ngược lại (xuống dốc) từng bước nhỏ để tới đáy — tức loss nhỏ nhất.

Mục tiêu (target) một bước, $y$, và loss:

$$
y=r+\gamma\,(1-d)\,\max_{a'}Q_{\theta^{-}}(s',a'),\qquad
\mathcal{L}(\theta)=\mathbb{E}_{(s,a,r,s',d)\sim\mathcal{D}}\big[\,\ell_{\delta}\!\big(y-Q_{\theta}(s,a)\big)\big].
$$

Giải nghĩa ký hiệu:

- $d\in\{0,1\}$ — cờ "đã kết thúc episode" (nếu kết thúc, không cộng giá trị tương lai).
- $\theta^{-}$ — tham số của **target network** (xem F.3).
- $\mathcal{D}$ — **replay buffer** (xem F.2).
- $\ell_\delta$ — hàm **Huber** (xem F.4).

### F.2 Mẹo 1 — Replay buffer (bộ nhớ phát lại)

Các bước liên tiếp trong thị trường **rất giống nhau** (tương quan cao). Học theo đúng thứ tự thời gian khiến mạng "thiên vị" đoạn gần nhất.

> **Giải pháp & ví dụ:** lưu lại các trải nghiệm $(s,a,r,s',d)$ vào một bộ nhớ, rồi mỗi lần học **bốc ngẫu nhiên** một nắm ra. Giống như ôn thi bằng cách **xáo trộn thẻ flashcard** thay vì học thuộc lòng theo đúng thứ tự trang sách.

Ở đây buffer chứa tối đa $10\,000$ trải nghiệm; mỗi lần học bốc ngẫu nhiên $64$ cái (gọi là một *minibatch*); chỉ bắt đầu học khi đã có $\ge 200$ trải nghiệm.

### F.3 Mẹo 2 — Target network (mạng mục tiêu)

Trong công thức loss, "mục tiêu" $y$ lại chứa chính $Q$. Nếu dùng cùng một mạng vừa-đoán-vừa-làm-mục-tiêu, ta như **đuổi theo cái bóng của chính mình** — mục tiêu nhảy mỗi khi ta cập nhật → mất ổn định.

> **Giải pháp:** giữ một **bản sao đông cứng** $\theta^{-}$ của mạng để tính mục tiêu, và chỉ thỉnh thoảng mới đồng bộ lại ($\theta^{-}\leftarrow\theta$ mỗi $C=50$ bước cập nhật). Mục tiêu đứng yên đủ lâu để mạng đuổi kịp.

### F.4 Mẹo 3 — Double DQN (chống "lạc quan thái quá")

Phép $\max$ trong mục tiêu có xu hướng **thổi phồng** giá trị (vì luôn chọn cái lớn nhất, kể cả khi đó chỉ là nhiễu). Double DQN tách đôi vai trò:

$$
a^{*}=\arg\max_{a'}Q_{\theta}(s',a'),\qquad
y=r+\gamma\,(1-d)\,Q_{\theta^{-}}\!\big(s',\,a^{*}\big).
$$

- Mạng chính $\theta$ **chọn** hành động tốt nhất $a^{*}$.
- Mạng mục tiêu $\theta^{-}$ **chấm điểm** hành động đó.

Hai "ý kiến" độc lập → bớt lạc quan ảo.

### F.5 Mẹo 4 — Huber loss (chịu nhiễu tốt)

Bình phương sai số phạt **rất nặng** các điểm lệch lớn (outlier) → dễ bị giật. **Huber** dùng bình phương khi sai số nhỏ và tuyến tính khi sai số lớn → mượt và bền:

$$
\ell_{\delta}(x)=
\begin{cases}
\tfrac12\,x^{2}, & \lvert x\rvert\le\delta,\\[4pt]
\delta\big(\lvert x\rvert-\tfrac12\delta\big), & \lvert x\rvert>\delta,
\end{cases}
\qquad \delta=1\ (\texttt{smooth\_l1}).
$$

### F.6 Bước cập nhật

Dùng tối ưu hoá Adam, kèm "cắt" độ lớn gradient để tránh bước nhảy quá to:

$$
\theta\leftarrow\theta-\eta\,\widehat{\nabla_{\theta}\mathcal{L}},\qquad \big\lVert\nabla_{\theta}\mathcal{L}\big\rVert_2\le 1,\qquad \eta=10^{-3}.
$$

($\eta$ = tốc độ học; $\lVert\cdot\rVert_2$ = độ dài vector gradient.)

---

## Phần G. Khám phá vs khai thác (ε-greedy)

Nếu agent **luôn** chọn nước nó nghĩ là tốt nhất, nó sẽ không bao giờ thử cái mới và có thể bỏ lỡ chiến lược hay hơn. Cần cân bằng:

- **Khai thác (exploit):** dùng nước tốt nhất đã biết.
- **Khám phá (explore):** thỉnh thoảng thử ngẫu nhiên.

> **Ví dụ:** đi ăn — phần lớn chọn quán "tủ" (khai thác), thỉnh thoảng thử quán mới (khám phá) để biết đâu ngon hơn.

**ε-greedy:** với xác suất $\epsilon$ chọn ngẫu nhiên, còn lại chọn greedy:

$$
\pi_{\epsilon}(a\mid s)=
\begin{cases}
1-\epsilon+\dfrac{\epsilon}{\lvert\mathcal{A}\rvert}, & a=\arg\max_{a'}Q_{\theta}(s,a'),\\[8pt]
\dfrac{\epsilon}{\lvert\mathcal{A}\rvert}, & \text{ngược lại.}
\end{cases}
$$

Đầu khi train cho $\epsilon$ cao (khám phá nhiều), giảm dần về thấp (khai thác). Lúc dùng thật (inference) đặt $\epsilon=0$ (luôn greedy).

**Lịch giảm $\epsilon$ (tuyến tính, theo số bước, horizon thích ứng):**

$$
\epsilon(g)=\max\!\Big(\epsilon_{\min},\ \epsilon_{\max}-(\epsilon_{\max}-\epsilon_{\min})\,\tfrac{g}{G}\Big),\qquad \epsilon_{\max}=1.0,\ \epsilon_{\min}=0.05,
$$

với $g$ = số bước đã đi, và horizon $G$ tính theo **ngân sách bước thực tế của từng thị trường**:

$$
G=0.6\cdot E\cdot\sum_{i}\big(L_i-1\big),
$$

$E$ = số epoch, $L_i$ = độ dài episode thứ $i$. Nhờ tính theo dữ liệu thực, thị trường ít dữ liệu (GOLD) vẫn kịp giảm $\epsilon$ về $0.05$ thay vì kẹt ở mức gần ngẫu nhiên.

---

## Phần H. Thiết kế cụ thể trong dự án

### H.1 Trạng thái $s_t$ (33 chiều)

$$
s_t=\big[\,\phi_t\ \big\Vert\ (\rho_t,\ u_t,\ h_t)\,\big]\in\mathbb{R}^{33}.
$$

- $\phi_t\in\mathbb{R}^{30}$ — 30 đặc trưng kỹ thuật của thị trường (lag returns, MA ratios, RSI, StochRSI, Bollinger %B, MACD, volatility, ROC, momentum, volume ratio). Xem [features.md](features.md).
- $\rho_t\in\{0,1\}$ — đang giữ lệnh long (1) hay flat (0).
- $u_t=\dfrac{p_t}{p_{\text{entry}}}-1$ — lãi/lỗ chưa chốt (0 nếu đang flat).
- $h_t=\min\!\big(\tfrac{\text{số ngày giữ}}{30},1\big)$ — số ngày đã giữ, chuẩn hoá về $[0,1]$.

Ký hiệu $\Vert$ nghĩa là **ghép nối** hai vector. $\dim = 30+3 = 33$.

### H.2 Hành động & động học vị thế

$$
\mathcal{A}=\{0:\text{hold},\,1:\text{buy},\,2:\text{sell}\},\qquad
\rho_t=
\begin{cases}
1, & a_t=\text{buy},\ \rho_{t-1}=0,\\
0, & a_t=\text{sell},\ \rho_{t-1}=1,\\
\rho_{t-1}, & \text{ngược lại.}
\end{cases}
$$

Mô hình **long-only, all-in/all-out**: hoặc giữ toàn bộ vốn ở lệnh, hoặc về tiền mặt.

### H.3 Phần thưởng

$$
r_{t+1}=\underbrace{\rho_t\!\left(\frac{p_{t+1}}{p_t}-1\right)}_{\text{lãi/lỗ theo vị thế}}\ -\ \underbrace{c\,\big\lvert\rho_t-\rho_{t-1}\big\rvert}_{\text{phí khi vào/ra lệnh}}\ -\ \underbrace{\lambda\,\rho_t}_{\text{phạt ôm long}}.
$$

- $\dfrac{p_{t+1}}{p_t}-1$ = phần trăm thay đổi giá ngày kế tiếp. Nếu đang long ($\rho_t=1$) và giá lên → thưởng dương; giá xuống → thưởng âm.
- $c=0.0005$ — phí, chỉ trừ khi đổi trạng thái (vào hoặc ra lệnh).
- $\lambda=0.0001$ — phạt nhỏ mỗi bước đang long, để agent không "ôm long vĩnh viễn".

---

## Phần I. Mạng Q và thuật toán huấn luyện

### I.1 Mạng (MLP 3 lớp)

$$
\begin{aligned}
h^{(1)}&=\mathrm{ReLU}\!\big(W_1 s+b_1\big), & W_1&\in\mathbb{R}^{128\times 33},\\
h^{(2)}&=\mathrm{ReLU}\!\big(W_2 h^{(1)}+b_2\big), & W_2&\in\mathbb{R}^{64\times 128},\\
Q_{\theta}(s,\cdot)&=W_3 h^{(2)}+b_3, & W_3&\in\mathbb{R}^{3\times 64}.
\end{aligned}
$$

- $W_k,b_k$ — ma trận trọng số và vector bias (chính là $\theta$ cần học).
- $\mathrm{ReLU}(x)=\max(0,x)$ — hàm kích hoạt, giữ phần dương, bỏ phần âm; giúp mạng học quan hệ phi tuyến.
- Output 3 số = $Q$ cho hold/buy/sell.

### I.2 Pseudocode (Double DQN)

```
Khởi tạo policy θ, target θ⁻ ← θ, buffer D rỗng, đếm bước g = 0
Tính horizon ε:  G = 0.6 · E · Σ_i (L_i − 1)        (E = 12 epoch)

lặp E epoch:
    xáo trộn các episode (chuỗi thật + chuỗi "gương", đã cắt cửa sổ)
    với mỗi episode:
        ρ ← 0 (flat)
        với mỗi bước t trong episode:
            s   ← [đặc trưng_t ‖ (ρ, lãi chưa chốt, ngày giữ)]
            ε   ← ε(g);  g ← g + 1
            a   ← ngẫu nhiên nếu may rủi < ε, ngược lại argmax_a Q_θ(s,a)
            cập nhật ρ theo a;  tính thưởng r
            s'  ← trạng thái kế tiếp
            lưu (s, a, r, s', done) vào D
            nếu |D| ≥ 200: lặp ≤ 10 lần:
                bốc ngẫu nhiên 64 mẫu từ D
                y ← mục tiêu Double DQN
                hạ loss Huber bằng Adam (cắt ‖∇‖ ≤ 1)
                mỗi 50 bước cập nhật: θ⁻ ← θ
    ghi log: ε, loss trung bình, tổng thưởng
Đánh giá greedy (ε=0) trên chuỗi thật → ghi greedy_total_reward
Lưu checkpoint rl_dqn_{market}.pt
```

---

## Phần J. Bốn cải tiến quan trọng (v2)

Bản đầu học kém: trên dữ liệu **toàn tăng**, agent học "cứ mua là thắng" và **không biết xử lý khi giá giảm**; thị trường ít dữ liệu thì $\epsilon$ kẹt ở mức ngẫu nhiên. Bốn cải tiến:

### J.1 Mirror augmentation — *dạy cho agent thấy "thị trường giảm"*

Vấn đề lớn nhất: nếu lịch sử chỉ toàn đi lên, agent **chưa từng trải nghiệm** một downtrend kéo dài, nên khi giá thật sự sập nó không biết phản ứng.

**Cách trị:** với mỗi chuỗi giá thật, tạo thêm một **chuỗi "gương"** đi xuống mỗi khi chuỗi gốc đi lên. Dùng log-return $r_t=\ln\frac{p_t}{p_{t-1}}$ rồi **đảo dấu**:

$$
\tilde p_0=p_0,\qquad \tilde p_t=\tilde p_{t-1}\,e^{-r_t}.
$$

Khai triển ra dạng đóng (chuỗi gương là **ảnh phản chiếu** của chuỗi gốc qua $p_0$):

$$
\tilde p_t=p_0\exp\!\Big(-\!\sum_{k=1}^{t}r_k\Big)=\frac{p_0^{2}}{p_t}.
$$

Với một lệnh long, phần thưởng trên chuỗi gương **đảo dấu** so với chuỗi gốc:

$$
\frac{\tilde p_{t+1}}{\tilde p_t}-1=e^{-r_{t+1}}-1\approx-\Big(\frac{p_{t+1}}{p_t}-1\Big).
$$

Kết quả: nửa số kinh nghiệm là thị trường giảm → agent học rằng *giữ long khi giảm thì bị phạt* → biết đứng ngoài/thoát lệnh khi xu hướng xuống. Hết thiên lệch "luôn mua".

### J.2 Sliding-window — *biến ít dữ liệu thành nhiều bài tập*

Thay vì coi mỗi chuỗi là một bài tập dài, ta **cắt nó thành nhiều đoạn ngắn chồng lấn** — mỗi đoạn là một episode riêng. Số cửa sổ từ một chuỗi dài $L$:

$$
n_{\text{win}}=\left\lfloor\frac{L-W}{S}\right\rfloor+1,\qquad W=130\ (\text{độ dài}),\ S=35\ (\text{bước nhảy}).
$$

($\lfloor\cdot\rfloor$ = làm tròn xuống.) Tổng episode (thật + gương) bị chặn trên bởi $400$. Nhờ vậy GOLD/CRYPTO (ít mã) vẫn có nhiều episode để luyện.

### J.3 ε giảm thích ứng — đã nói ở **Phần G** (horizon $G$ theo số bước thực tế).

### J.4 Đánh giá greedy — *chấm điểm công bằng*

Phần thưởng ghi trong lúc train bị nhiễu vì agent đang khám phá ngẫu nhiên. Sau khi train xong, ta chạy một lượt **không khám phá** ($\epsilon=0$) trên các chuỗi **thật** và cộng phần thưởng:

$$
J_{\text{greedy}}=\sum_{\text{chuỗi thật}}\ \sum_{t}\,r_{t+1}\Big|_{\,a_t=\arg\max_a Q_{\theta}(s_t,a)}.
$$

Con số này (`greedy_total_reward`) mới phản ánh **chất lượng thật** của chính sách. Mỗi epoch cũng ghi `avg_loss` để xem mạng có hội tụ không.

---

## Phần K. Hai vai trò khi dùng

### K.1 Vai trò "thuật toán dự đoán" — đổi hành động thành giá

Để hợp với khung 11 thuật toán kia (vốn xuất ra một con số giá), ta đổi hành động của agent thành `predicted_price`. Đầu tiên đo độ biến động gần đây bằng độ lệch chuẩn của log-return (cửa sổ $n=20$):

$$
\sigma_t=\sqrt{\frac{1}{n-1}\sum_{k=t-n+1}^{t}\big(r_k-\bar r\big)^{2}}.
$$

Rồi ánh xạ hành động → giá ($\kappa=1.5$):

$$
\hat p_{t+1}=
\begin{cases}
p_t\,(1+\kappa\sigma_t), & \text{buy (dự đoán lên)},\\
p_t\,(1-\kappa\sigma_t), & \text{sell (dự đoán xuống)},\\
p_t\,(1+0.1\,\sigma_t), & \text{hold (gần như đứng yên)},
\end{cases}
$$

kẹp trong giới hạn cho phép của thị trường ($m$):

$$
\hat p_{t+1}\leftarrow\mathrm{clip}\!\big(\hat p_{t+1},\ p_t(1-m),\ p_t(1+m)\big).
$$

Độ tin cậy = xác suất của hành động được chọn, tính bằng **softmax** (biến 3 con số $Q$ thành 3 xác suất cộng lại bằng 1):

$$
\text{confidence}=\big[\mathrm{softmax}\,Q_\theta(s)\big]_a=\frac{e^{Q_\theta(s)_a}}{\sum_{a'}e^{Q_\theta(s)_{a'}}}.
$$

### K.2 Vai trò "bot giao dịch native"

Ở tầng simulation, bot `{market}_rl_dqn` (1 bot/thị trường) **để policy tự quyết** mua/bán/giữ, không dùng luật ngưỡng:

```
SL/TP safety check (chốt lời/cắt lỗ — lưới an toàn cứng)
với mỗi mã của thị trường:
    s = [ đặc trưng(giá ≤ ngày sim)  ‖  tình trạng vị thế THẬT của bot ]
    a = argmax_a Q_θ(s)      (greedy, ε = 0)
    thực thi BUY/SELL/HOLD trực tiếp
```

Backtest cần giá "as-of" (`get_*_prices_asc_as_of`) để dựng trạng thái đúng thời điểm quá khứ.

---

## Phần L. Lưu model & dự phòng

- Huấn luyện xong, lưu trọng số ra đĩa: `${RL_MODEL_DIR}/rl_dqn_{market}.pt` (mỗi thị trường một file). Train per-market qua `train_for_market()` / `train_single_algorithm()`.
- Lần gọi đầu sẽ **nạp lại** checkpoint → khởi động lại service không mất model.
- Thiếu PyTorch, thiếu checkpoint, hoặc lỗi → **dự phòng EMA** (trung bình động mượt), `confidence = 0.35`; `act()` trả về hold. Pipeline không bao giờ vỡ.

$$
\text{EMA}_t=\alpha\,p_t+(1-\alpha)\,\text{EMA}_{t-1},\quad \alpha=\frac{2}{N+1},\quad
\hat p_{t+1}=p_t\Big(1+0.5\cdot\tfrac{\text{EMA}-p_t}{p_t}\Big).
$$

---

## Phần M. Bảng hyperparameter

| Nhóm | Tham số | Giá trị |
|---|---|---|
| Mạng | HIDDEN1 / HIDDEN2 / N_ACTIONS | 128 / 64 / 3 |
| Mạng | in_dim (30 features + 3 vị thế) | 33 |
| RL | GAMMA (γ, chiết khấu) | 0.99 |
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
| Augment | MAX_EPISODES_PER_MARKET | 400 |
| Predict | K_SIGMA (κ) / SIGMA_WINDOW (n) | 1.5 / 20 |
| Data | MIN_DATA_POINTS | 80 |
| Loss | Huber δ / clip-norm | 1.0 / 1.0 |

---

## Phần N. Đánh giá chất lượng (đừng nhìn mỗi reward train)

Reward lúc train chỉ là proxy. Thước đo thật:

**Direction accuracy** — tỉ lệ đoán đúng hướng (lên/xuống), tính sau khi đối chiếu giá thật:

$$
\text{DA}=\frac{1}{N}\sum_{i=1}^{N}\mathbf{1}\!\left[\,\mathrm{sign}(\hat p_i-p_i)=\mathrm{sign}(p_i^{\text{actual}}-p_i)\,\right].
$$

($\mathbf{1}[\cdot]$ = 1 nếu điều trong ngoặc đúng, ngược lại 0; $\mathrm{sign}$ = dấu.)

**Hiệu năng bot** — tỉ số Sharpe (lợi nhuận trên rủi ro) và sụt giảm tối đa:

$$
\text{Sharpe}=\frac{\mathbb{E}[R]}{\sqrt{\operatorname{Var}[R]}}\sqrt{P},\qquad
\text{MaxDD}=\max_{t}\Big(1-\frac{V_t}{\max_{\tau\le t}V_\tau}\Big),
$$

cùng win-rate, profit factor (xem trang Simulation / Monitoring).

---

## Phần O. Điểm mạnh / yếu

**Mạnh**
- Học thẳng **chính sách giao dịch** (tối ưu tiền lãi), không chỉ đoán số.
- Trạng thái có chứa vị thế ⇒ biết mình đang giữ gì để quyết.
- Mirror augmentation ⇒ biết cả thị trường lên lẫn xuống, không "luôn mua".
- Lưu checkpoint ⇒ chạy nhanh, khởi động lại không mất.

**Yếu / giới hạn**
- Cần nhiều dữ liệu đa dạng; GOLD/CRYPTO ít mã ⇒ thưởng nhỏ, giao dịch dè dặt.
- Long-only, all-in/all-out — chưa short, chưa chia tỉ lệ vốn.
- Mạng nhìn "ảnh chụp" đặc trưng — chưa khai thác cấu trúc chuỗi như LSTM/GRU.
- Nhạy hyperparameter; phải xác nhận chất lượng qua DA + PnL theo thời gian, không tin mỗi reward train.

---

## Phần P. Dependency

- `torch` (PyTorch) — bắt buộc; thiếu thì dự phòng EMA.
- `numpy`.
- `build_enhanced_features` (xem [features.md](features.md)).
