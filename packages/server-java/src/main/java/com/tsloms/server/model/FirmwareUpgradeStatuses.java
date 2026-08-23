// 固件包与升级记录实体：对齐 Go 版 internal/model/firmware.go。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.PrePersist;
import jakarta.persistence.PreUpdate;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import java.time.Instant;

/** 固件升级状态常量。 */
public final class FirmwareUpgradeStatuses {
    public static final String PENDING = "pending";
    public static final String UPGRADING = "upgrading";
    public static final String SUCCESS = "success";
    public static final String FAILED = "failed";

    private FirmwareUpgradeStatuses() {
    }
}
