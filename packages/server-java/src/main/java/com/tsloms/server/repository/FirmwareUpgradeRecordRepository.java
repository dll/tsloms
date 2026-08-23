// 升级记录仓库。
package com.tsloms.server.repository;

import com.tsloms.server.model.FirmwareUpgradeRecord;
import org.springframework.data.jpa.repository.JpaRepository;

public interface FirmwareUpgradeRecordRepository
        extends JpaRepository<FirmwareUpgradeRecord, Long> {
}
