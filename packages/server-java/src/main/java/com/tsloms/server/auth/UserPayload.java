// 用户信息载荷：字段名逐项对齐 Go 版 handler/auth.go userPayload（前端契约）。
package com.tsloms.server.auth;

import com.tsloms.server.model.User;
import java.util.LinkedHashMap;
import java.util.Map;

public final class UserPayload {

    /** 构建与 Go 版 gin.H 完全同键的用户信息 Map。 */
    public static Map<String, Object> of(User u) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", u.id);
        m.put("username", u.username);
        m.put("role", u.role);
        m.put("real_name", u.realName);
        m.put("phone", u.phone);
        m.put("phone_login", u.phoneLogin);
        m.put("phone_verified", u.phoneVerified);
        m.put("email", u.email);
        m.put("department_id", u.departmentId);
        m.put("status", u.status);
        m.put("center_lat", u.centerLat);
        m.put("center_lng", u.centerLng);
        m.put("work_no", u.workNo);
        m.put("avatar", u.avatar);
        m.put("gender", u.gender);
        m.put("id_card", u.idCard);
        m.put("address", u.address);
        m.put("education", u.education);
        m.put("engineer_level", u.engineerLevel);
        return m;
    }

    private UserPayload() {
    }
}
