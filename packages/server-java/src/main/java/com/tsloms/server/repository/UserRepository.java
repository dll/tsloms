// 用户仓库：常用查询对齐 handler 层既有用法。
package com.tsloms.server.repository;

import com.tsloms.server.model.User;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface UserRepository
        extends JpaRepository<User, Long>, JpaSpecificationExecutor<User> {

    Optional<User> findByUsername(String username);

    Optional<User> findByPhoneLogin(String phoneLogin);

    boolean existsByUsername(String username);
}
