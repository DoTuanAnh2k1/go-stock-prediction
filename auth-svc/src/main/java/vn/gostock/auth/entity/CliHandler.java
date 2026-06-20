package vn.gostock.auth.entity;

import jakarta.persistence.*;
import lombok.*;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

@Entity
@Table(name = "cli_handlers")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class CliHandler {
    @Id
    @Column(name = "handler_key", length = 80)
    private String handlerKey;

    @Column(name = "display_name", nullable = false, length = 120)
    private String displayName;

    @Column(nullable = false, length = 10)
    private String verb;

    @Column(nullable = false, length = 40)
    private String resource;

    // JSON array string, stored as jsonb
    @JdbcTypeCode(SqlTypes.JSON)
    @Column(name = "arg_schema", columnDefinition = "jsonb")
    @Builder.Default
    private String argSchema = "[]";

    @Column(nullable = false)
    @Builder.Default
    private Boolean enabled = true;
}
