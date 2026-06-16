package vn.gostock.auth.entity;

import jakarta.persistence.*;
import lombok.*;
import java.time.LocalDateTime;

@Entity
@Table(name = "users")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class User {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, unique = true, length = 50)
    private String username;

    @Column(name = "password_hash", nullable = false)
    private String passwordHash;

    // valid: "super_admin", "admin", "user"
    @Column(nullable = false)
    private String role;

    @Column(name = "full_name", length = 100)
    private String fullName;

    @Column(length = 255)
    private String email;

    @Column(length = 30)
    private String phone;

    @Column(name = "created_at")
    private LocalDateTime createdAt;

    @Column(name = "updated_at")
    private LocalDateTime updatedAt;

    // GORM soft-delete compatibility — always filter WHERE deleted_at IS NULL
    @Column(name = "deleted_at")
    private LocalDateTime deletedAt;
}
