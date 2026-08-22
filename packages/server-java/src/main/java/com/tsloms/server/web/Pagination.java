// 分页参数解析：对齐 Go 版 handler/response.go paginate（默认 1/20，上限 100）。
package com.tsloms.server.web;

import jakarta.servlet.http.HttpServletRequest;

public final class Pagination {

    public static final int DEFAULT_PAGE_SIZE = 20;
    public static final int MAX_PAGE_SIZE = 100;

    /** 解析后的分页参数。 */
    public record Page(int page, int pageSize) {
        public int offset() {
            return (page - 1) * pageSize;
        }
    }

    private Pagination() {
    }

    /** 从查询参数 page/page_size 解析，非法值回退默认。 */
    public static Page of(HttpServletRequest request) {
        return of(request.getParameter("page"), request.getParameter("page_size"));
    }

    public static Page of(String pageStr, String pageSizeStr) {
        int page = parseIntOrDefault(pageStr, 1);
        int pageSize = parseIntOrDefault(pageSizeStr, DEFAULT_PAGE_SIZE);
        if (page < 1) {
            page = 1;
        }
        if (pageSize < 1) {
            pageSize = DEFAULT_PAGE_SIZE;
        }
        if (pageSize > MAX_PAGE_SIZE) {
            pageSize = MAX_PAGE_SIZE;
        }
        return new Page(page, pageSize);
    }

    private static int parseIntOrDefault(String s, int def) {
        if (s == null || s.isBlank()) {
            return def;
        }
        try {
            return Integer.parseInt(s.trim());
        } catch (NumberFormatException e) {
            return def;
        }
    }
}
