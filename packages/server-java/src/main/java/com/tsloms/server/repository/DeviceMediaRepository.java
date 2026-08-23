// 设备媒体仓库。
package com.tsloms.server.repository;

import com.tsloms.server.model.DeviceMedia;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;

public interface DeviceMediaRepository
        extends JpaRepository<DeviceMedia, Long>, JpaSpecificationExecutor<DeviceMedia> {
}
