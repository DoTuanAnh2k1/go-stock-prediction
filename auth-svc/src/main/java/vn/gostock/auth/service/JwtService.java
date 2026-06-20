package vn.gostock.auth.service;

import com.auth0.jwt.JWT;
import com.auth0.jwt.algorithms.Algorithm;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import vn.gostock.auth.entity.User;
import java.util.Date;
import java.util.List;

@Service
public class JwtService {

    @Value("${jwt.secret}")
    private String secret;

    @Value("${jwt.expiration-ms:86400000}")
    private long expirationMs;

    public String generateToken(User user, List<String> accessibleMarkets) {
        Algorithm algorithm = Algorithm.HMAC256(secret);
        return JWT.create()
            .withSubject(String.valueOf(user.getId()))
            .withClaim("username", user.getUsername())
            .withClaim("role", user.getRole())
            .withClaim("user_id", user.getId())
            .withClaim("accessible_markets", accessibleMarkets)
            .withIssuedAt(new Date())
            .withExpiresAt(new Date(System.currentTimeMillis() + expirationMs))
            .sign(algorithm);
    }
}
