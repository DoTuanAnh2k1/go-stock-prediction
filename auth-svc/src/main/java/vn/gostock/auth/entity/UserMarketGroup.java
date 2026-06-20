package vn.gostock.auth.entity;

import jakarta.persistence.*;
import lombok.*;

@Entity
@Table(name = "user_market_groups")
@Data
@NoArgsConstructor
@AllArgsConstructor
@IdClass(UserMarketGroupId.class)
public class UserMarketGroup {
    @Id
    @Column(name = "user_id")
    private Long userId;

    @Id
    @Column(name = "group_id")
    private Long groupId;
}
