// 区划层级与路口状态常量（对齐 Go 版 model/crossing.go 常量组）。
package com.tsloms.server.model;

public final class AreaTypes {
    public static final String PROVINCE = "province";
    public static final String CITY = "city";
    public static final String DISTRICT = "district";
    public static final String STREET = "street";
    public static final String COMMUNITY = "community";
    public static final String ROAD = "road";

    private AreaTypes() {
    }
}
