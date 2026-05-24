# Kế hoạch sửa chữa Go Stock Prediction

## Tổng quan

Project hiện là proof-of-concept có kiến trúc tốt nhưng nhiều lỗi nghiêm trọng.
Kế hoạch chia thành 5 phase, ưu tiên từ critical → production-ready.

## Các phase

| Phase | Tên | Mức độ ưu tiên | File kế hoạch |
|-------|-----|----------------|---------------|
| 1 | Critical Bug Fixes | **PHẢI LÀM NGAY** | [phase1-critical-bugs.md](phase1-critical-bugs.md) |
| 2 | Security & Input Validation | Cao | [phase2-security.md](phase2-security.md) |
| 3 | ML Models Thực Sự | Trung bình | [phase3-ml-models.md](phase3-ml-models.md) |
| 4 | Testing | Trung bình | [phase4-testing.md](phase4-testing.md) |
| 5 | Performance & Production | Thấp | [phase5-production.md](phase5-production.md) |

## Thứ tự thực hiện

```
Phase 1 (bugs) → Phase 2 (security) → Phase 3 (ML) → Phase 4 (tests) → Phase 5 (prod)
```

Phase 1 và 2 phải làm trước vì app hiện tại có thể crash hoặc bị exploit.
Phase 3 là cốt lõi chức năng dự đoán — nếu bỏ qua thì accuracy claims là vô nghĩa.
Phase 4 và 5 là để đưa lên production thực sự.

## Trạng thái hiện tại

- [ ] Phase 1 — chưa bắt đầu
- [ ] Phase 2 — chưa bắt đầu
- [ ] Phase 3 — chưa bắt đầu
- [ ] Phase 4 — chưa bắt đầu
- [ ] Phase 5 — chưa bắt đầu
