// 固件包仓库：含发布状态过滤 Specification。
package com.tsloms.server.repository;

import com.tsloms.server.model.FirmwarePackage;
import org.springframework.data.jpa.domain.Specification;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface FirmwarePackageRepository
        extends JpaRepository<FirmwarePackage, Long>, JpaSpecificationExecutor<FirmwarePackage> {

    boolean existsByVersion(String version);

    /** 过滤：仅已发布。 */
    Specification<FirmwarePackage> PUBLISHED =
            (root, query, cb) -> cb.isTrue(root.get("published"));

    /** 无过滤。 */
    Specification<FirmwarePackage> ALL = (root, query, cb) -> cb.conjunction();
}
