// 分页参数解析单元测试（对齐 Go 版 paginate 边界）。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;

import com.tsloms.server.web.Pagination;
import org.junit.jupiter.api.Test;

class PaginationTest {

    @Test
    void 默认值与上限() {
        assertThat(Pagination.of(null, null).page()).isEqualTo(1);
        assertThat(Pagination.of(null, null).pageSize()).isEqualTo(20);
        // 非法值回退
        assertThat(Pagination.of("abc", "xyz").pageSize()).isEqualTo(20);
        // 越界修正
        assertThat(Pagination.of("-5", "-1").page()).isEqualTo(1);
        assertThat(Pagination.of("-5", "-1").pageSize()).isEqualTo(20);
        assertThat(Pagination.of("2", "999").pageSize()).isEqualTo(100); // 上限
        assertThat(Pagination.of("3", "50").offset()).isEqualTo(100);
    }
}
