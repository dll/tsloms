// 设备仓库：在线统计等。
package com.tsloms.server.repository;

import com.tsloms.server.model.Device;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

public interface DeviceRepository extends JpaRepository<Device, Long> {

    Optional<Device> findByHwId(String hwId);

    long countByOnlineStatus(boolean onlineStatus);

    boolean existsByHwId(String hwId);
}
