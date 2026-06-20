package vn.gostock.auth.entity;

import lombok.*;
import java.io.Serializable;

@Data
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode
public class UserCommandGroupId implements Serializable {
    private Long userId;
    private Long groupId;
}
