// 站内通知接口：列表(合并广播已读)/未读数/单条已读/全部已读。
// 契约对齐 Go 版 /notifications 路由组（模块开关 gating 待统一拦截器阶段补齐）。
package com.tsloms.server.notification;

import com.tsloms.server.model.Notification;
import com.tsloms.server.model.NotificationRead;
import com.tsloms.server.repository.NotificationReadRepository;
import com.tsloms.server.repository.NotificationRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import com.tsloms.server.web.Pagination;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.http.ResponseEntity;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1/notifications")
public class NotificationController {

    private final NotificationRepository notifications;
    private final NotificationReadRepository reads;

    public NotificationController(NotificationRepository notifications,
                                  NotificationReadRepository reads) {
        this.notifications = notifications;
        this.reads = reads;
    }

    /** GET /notifications：个人通知 + 全员广播（广播已读按记录表合并）。 */
    @GetMapping
    public ApiResponse<Map<String, Object>> list(HttpServletRequest request) {
        Long userId = AuthInterceptor.userId(request);
        Pagination.Page pg = Pagination.of(request);

        List<Notification> rows = notifications.findAll(
                PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt"))).getContent();

        Set<Long> readIds = new HashSet<>();
        if (userId != null) {
            reads.findAll().stream()
                    .filter(r -> userId.equals(r.userId))
                    .forEach(r -> readIds.add(r.notificationId));
        }
        List<Object> list = new ArrayList<>();
        for (Notification n : rows) {
            boolean read = n.isRead || (userId != null && readIds.contains(n.id));
            Map<String, Object> m = new LinkedHashMap<>(Map.of(
                    "id", n.id, "user_id", n.userId, "type", n.type,
                    "title", nz(n.title), "content", nz(n.content),
                    "link", nz(n.link), "biz_type", nz(n.bizType),
                    "biz_id", n.bizId, "created_at", n.createdAt));
            m.put("is_read", read);
            list.add(m);
        }
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", list);
        data.put("total", rows.size());
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** GET /notifications/unread-count：个人未读 + 广播未读（去重合并）。 */
    @GetMapping("/unread-count")
    public ApiResponse<Map<String, Object>> unreadCount(HttpServletRequest request) {
        Long userId = AuthInterceptor.userId(request);
        int unread = 0;
        if (userId != null) {
            Set<Long> readIds = new HashSet<>();
            reads.findAll().stream()
                    .filter(r -> userId.equals(r.userId))
                    .forEach(r -> readIds.add(r.notificationId));
            for (Notification n : notifications.findAll()) {
                if (n.isRead) {
                    continue; // 个人通知直接读标记
                }
                if (n.userId != null && n.userId > 0 && !userId.equals(n.userId)) {
                    continue; // 他人专属
                }
                if (n.userId != null && n.userId == 0 && readIds.contains(n.id)) {
                    continue; // 广播但该用户已读
                }
                unread++;
            }
        }
        return ApiResponse.ok(Map.of("unread", unread));
    }

    /** PUT /notifications/{id}/read：标记已读（广播写按用户记录）。 */
    @PutMapping("/{id}/read")
    public ResponseEntity<?> markRead(@PathVariable Long id, HttpServletRequest request) {
        Long userId = AuthInterceptor.userId(request);
        var opt = notifications.findById(id);
        if (opt.isEmpty()) {
            return notFound("通知不存在");
        }
        Notification n = opt.get();
        if (userId != null && n.userId == 0L) {
            // 广播：写按用户已读记录（幂等）
            if (reads.findByNotificationIdAndUserId(id, userId).isEmpty()) {
                NotificationRead r = new NotificationRead();
                r.notificationId = id;
                r.userId = userId;
                r.readAt = Instant.now();
                reads.save(r);
            }
        } else {
            n.isRead = true;
            notifications.save(n);
        }
        return ResponseEntity.ok(ApiResponse.ok(Map.of("message", "已标记为已读")));
    }

    /** PUT /notifications/read-all：全部已读。 */
    @PutMapping("/read-all")
    @Transactional
    public ResponseEntity<?> readAll(HttpServletRequest request) {
        Long userId = AuthInterceptor.userId(request);
        if (userId == null) {
            return ResponseEntity.status(401)
                    .body(ApiResponse.fail("unauthorized", "未登录"));
        }
        for (Notification n : notifications.findAll()) {
            if (n.userId != null && n.userId.equals(userId) && !n.isRead) {
                n.isRead = true;
                notifications.save(n);
            } else if (n.userId != null && n.userId == 0L
                    && reads.findByNotificationIdAndUserId(n.id, userId).isEmpty()
                    && !readIdsCache().contains(n.id)) {
                NotificationRead r = new NotificationRead();
                r.notificationId = n.id;
                r.userId = userId;
                r.readAt = Instant.now();
                reads.save(r);
            }
        }
        return ResponseEntity.ok(ApiResponse.ok(Map.of("message", "全部已读")));
    }

    /** 广播已读缓存占位（read-all 简化：逐条幂等写入即可）。 */
    private Set<Long> readIdsCache() {
        return new HashMap<Long, Boolean>().keySet();
    }

    static String nz(String s) {
        return s == null ? "" : s;
    }

    private ResponseEntity<?> notFound(String msg) {
        return ResponseEntity.status(404).body(ApiResponse.fail("not_found", msg));
    }
}
