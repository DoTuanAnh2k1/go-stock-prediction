package vn.gostock.auth.entity;

import jakarta.persistence.*;
import lombok.*;

@Entity
@Table(name = "user_command_groups")
@Data
@NoArgsConstructor
@AllArgsConstructor
@IdClass(UserCommandGroupId.class)
public class UserCommandGroup {
    @Id
    @Column(name = "user_id")
    private Long userId;

    @Id
    @Column(name = "group_id")
    private Long groupId;
}
