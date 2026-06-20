package vn.gostock.auth.entity;

import jakarta.persistence.*;
import lombok.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;

@Entity
@Table(name = "market_groups")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class MarketGroup {
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, unique = true, length = 100)
    private String name;

    @Column(columnDefinition = "TEXT")
    private String description;

    @Column(name = "created_at")
    private LocalDateTime createdAt;

    @Column(name = "updated_at")
    private LocalDateTime updatedAt;

    @ElementCollection(fetch = FetchType.EAGER)
    @CollectionTable(
        name = "market_group_markets",
        joinColumns = @JoinColumn(name = "group_id")
    )
    @Column(name = "market_key")
    @Builder.Default
    private List<String> marketKeys = new ArrayList<>();
}
