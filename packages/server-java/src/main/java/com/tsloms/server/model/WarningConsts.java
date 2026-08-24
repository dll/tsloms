// 预警实体与规则实体：对齐 Go 版 model/warning.go。
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
import java.time.Instant;

public final class WarningConsts {
    public static final String LEVEL_CRITICAL = "critical";
    public static final String LEVEL_WARNING = "warning";
    public static final String LEVEL_INFO = "info";

    public static final String DEAL_UNHANDLED = "unhandled";
    public static final String DEAL_IGNORED = "ignored";
    public static final String DEAL_RESOLVED = "resolved";

    public static final String UNTRANSFERRED = "untransferred";
    public static final String TRANSFERRED = "transferred";

    public static final String SOURCE_FAULT = "fault";
    public static final String SOURCE_MQTT = "mqtt";
    public static final String SOURCE_SELFCHECK = "selfcheck";
    public static final String SOURCE_MANUAL = "manual";

    private WarningConsts() {
    }
}
