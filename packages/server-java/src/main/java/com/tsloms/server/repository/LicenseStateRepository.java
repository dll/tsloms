// LicenseStateRepository ：LicenseState 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.LicenseState;
import org.springframework.data.jpa.repository.JpaRepository;

public interface LicenseStateRepository extends JpaRepository<LicenseState, Long> {
}
