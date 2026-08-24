// 区划仓库：树构建查询。
package com.tsloms.server.repository;

import com.tsloms.server.model.Area;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

public interface AreaRepository extends JpaRepository<Area, Long> {

    List<Area> findByParentIdOrderByIdAsc(Long parentId);

    List<Area> findByParentIdIsNullOrderByAreaSortAscIdAsc();

    List<Area> findByAreaTypeOrderByAreaSortAscIdAsc(String areaType);
}
