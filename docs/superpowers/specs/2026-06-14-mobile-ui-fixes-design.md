# Mobile UI Fixes — Design Spec

**Date:** 2026-06-14  
**Status:** Approved

## Problem

1. **Horizontal overflow** — một số trang yêu cầu scroll ngang trên điện thoại vì table không có wrapper, và topbar quá chật.
2. **Thiếu navigation sau login** — `MobNav` (bottom nav) hardcode 7 item, không có Settings hay Data Pipeline dù user đã đăng nhập. Sidebar bị ẩn hoàn toàn trên mobile.

---

## Fix 1: Table overflow

### Root cause

Sau khi kiểm tra toàn bộ 23 `<table>` trong codebase, hầu hết đã có `<div style={{overflowX:'auto'}}>` wrapper. Chỉ còn 3 nơi thiếu:

| File | Line | Mô tả |
|------|------|--------|
| `frontend/src/pages/MarketPredictions.tsx` | 267 | Bảng dự đoán theo market |
| `frontend/src/pages/MarketTraining.tsx` | 189 | Bảng lịch sử training |
| `frontend/src/pages/Users.tsx` | 142 | Bảng danh sách users |

### Solution

Bọc mỗi `<table>` bằng `<div style={{overflowX: 'auto'}}>` — nhất quán với pattern đã dùng ở các file khác.

---

## Fix 2: CSS — Content & Topbar mobile

### content overflow guard

Thêm vào `styles.css` trong block `@media (max-width: 599px)`:
```css
.content { overflow-x: hidden; }
```
Ngăn horizontal bleed thoát ra ngoài layout ở cấp cao nhất.

### topbar mobile

Trên ≤599px, topbar còn 5-6 phần tử sau spacer: bell, lang-tog, theme-tog, auth. Với `gap: 16px` rất chật.

Thêm vào block `@media (max-width: 599px)`:
```css
.topbar { gap: 8px; }
.topbar .btn--icon { display: none; }   /* ẩn bell */
```

Bell là notification placeholder chưa hoạt động — ẩn đi phù hợp.  
Lang-tog và theme-tog giữ nguyên (nhỏ, hữu dụng).

---

## Fix 3: MobNav auth-aware

### File: `frontend/src/components/ui.tsx`

**Hàm `MobNav`** hiện chỉ render `getNav(t)` — danh sách cứng 7 item.

**Thay đổi:** `MobNav` gọi `useAuth()` để lấy `user`, sau đó build danh sách động:

```
Luôn hiện (7 item):
  Dashboard · Gold · NASDAQ · SP500 · Crypto · Simulation · Guide

+ khi user != null (2 item thêm):
  Monitoring  icon=activity   /monitoring
  Settings    icon=settings   /settings

+ khi user.role === 'admin' (1 item thêm):
  Users       icon=user       /admin/users
```

Bottom nav đã có `overflow-x: auto` + `scrollbar-width: none` → scroll ngang mượt khi nhiều item.

Không thay đổi gì về layout hay style của MobNav.

---

## Files sẽ thay đổi

| File | Loại thay đổi |
|------|---------------|
| `frontend/src/pages/MarketPredictions.tsx` | Bọc table với overflow-x wrapper |
| `frontend/src/pages/MarketTraining.tsx` | Bọc table với overflow-x wrapper |
| `frontend/src/pages/Users.tsx` | Bọc table với overflow-x wrapper |
| `frontend/src/styles.css` | Thêm 3 dòng CSS vào mobile media query |
| `frontend/src/components/ui.tsx` | MobNav dùng useAuth(), thêm item có điều kiện |

---

## Out of scope

- Không thay đổi desktop/tablet layout
- Không thêm animation hay visual mới vào MobNav
- Không thêm trang mới hay route mới
