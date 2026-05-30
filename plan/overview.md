# Plan Tổng Thể - Sửa Web Dashboard

## Tình trạng hiện tại

Dashboard có rất nhiều vấn đề nghiêm trọng ảnh hưởng đến khả năng sử dụng:
- UI lẫn lộn tiếng Việt không dấu và tiếng Anh
- Nhiều tính năng chỉ là placeholder (alert "Coming soon!")
- Dữ liệu sai/thiếu: Current Price = 0, Confidence = 0%, mock data
- Thiếu biểu đồ, thiếu trang chi tiết, thiếu pagination thật
- Giá vàng cũng tương tự - thiếu dữ liệu SJC
- Không có test nào trong toàn bộ project

## Phân chia Phase

**Mỗi phase bao gồm test đi kèm — không ship code chưa có test.**

| Phase | Tên | Mô tả | Ưu tiên |
|-------|-----|--------|---------|
| 0 | [Test Infrastructure](phase0-test-infra.md) | Setup test framework, fixtures, Makefile, CI | PREREQUISITE |
| 1 | [Critical Bugs](phase1-critical-bugs.md) | Sửa dữ liệu sai + test verify fix | CRITICAL |
| 2 | [Ngôn ngữ & UI](phase2-language-ui.md) | Thống nhất tiếng Việt có dấu + test encoding | HIGH |
| 3 | [Trang chi tiết](phase3-detail-pages.md) | Stock/prediction/training detail + API tests | HIGH |
| 4 | [Biểu đồ & Visualization](phase4-charts.md) | Charts + API data tests | MEDIUM |
| 5 | [Giá Vàng](phase5-gold.md) | Fix crawler, biểu đồ vàng + crawler tests | MEDIUM |
| 6 | [Hoàn thiện 3 Tab](phase6-tab-content.md) | Bỏ hardcode, thêm actual vs predicted, biểu đồ phân tích, training metrics | HIGH |

## Nguyên tắc chung

1. **Ngôn ngữ**: Toàn bộ UI chuyển sang tiếng Việt có dấu đầy đủ
2. **Không mock data**: Xóa hết mock/sample data, hiển thị trạng thái "chưa có dữ liệu" rõ ràng
3. **Backend trước, frontend sau**: Sửa API/logic trước, rồi mới sửa giao diện
4. **Test đi kèm code**: Mỗi phase PHẢI có test cho phần code thay đổi. Không merge nếu test fail.
5. **Definition of Done cho mỗi phase**: Code + Test + `go test ./...` pass

## Dependency & song song

```
Phase 0 (test infra)
    │
    ├──── Phase 1 (critical bugs) ──┐
    │                                ├──── Phase 3 (detail pages) ─┐
    └──── Phase 2 (ngôn ngữ UI) ───┘     │                        ├── Phase 4 (charts)
                                          ├── Phase 5 (giá vàng)  ─┘
                                          └── Phase 6 (3 tabs) ────┘
```

- Phase 1 // Phase 2: song song (backend logic vs UI text)
- Phase 3 // Phase 5 // Phase 6: song song (khác domain)
- Phase 4: cuối cùng (cần APIs từ Phase 3 + 6)

## Coverage target tích lũy

| Sau Phase | Target |
|-----------|--------|
| Phase 0 | Test infrastructure sẵn sàng, `make test` chạy được |
| Phase 1 | Core algorithms >= 80%, API prediction handler >= 70% |
| Phase 2 | Template rendering test (Go httptest) |
| Phase 3 | New API endpoints >= 80%, repository queries >= 70% |
| Phase 4 | Chart data APIs >= 70% |
| Phase 5 | Gold crawler >= 60%, gold APIs >= 70% |
| Phase 6 | New APIs >= 70%, accuracy calculation >= 80% |
| **Tổng** | **Overall >= 60%** |
