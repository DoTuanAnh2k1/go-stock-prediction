-- Flyway V2: thêm các trường profile cho bảng users
-- full_name, email, phone — đều nullable

ALTER TABLE users
    ADD COLUMN full_name VARCHAR(100) NULL,
    ADD COLUMN email     VARCHAR(255) NULL,
    ADD COLUMN phone     VARCHAR(30)  NULL;
