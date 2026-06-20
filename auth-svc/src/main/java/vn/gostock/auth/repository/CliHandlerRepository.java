package vn.gostock.auth.repository;

import org.springframework.data.jpa.repository.JpaRepository;
import vn.gostock.auth.entity.CliHandler;

public interface CliHandlerRepository extends JpaRepository<CliHandler, String> {
}
