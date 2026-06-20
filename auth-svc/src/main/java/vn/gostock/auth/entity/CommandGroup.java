package vn.gostock.auth.entity;

import jakarta.persistence.*;
import lombok.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;

@Entity
@Table(name = "command_groups")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class CommandGroup {
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

    // Membership of command ids — mirror MarketGroup.marketKeys element-collection mapping.
    @ElementCollection(fetch = FetchType.EAGER)
    @CollectionTable(
        name = "command_group_commands",
        joinColumns = @JoinColumn(name = "group_id")
    )
    @Column(name = "command_id")
    @Builder.Default
    private List<Long> commandIds = new ArrayList<>();
}
