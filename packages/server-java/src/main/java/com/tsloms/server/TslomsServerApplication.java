// TSLOMS 后端入口（Java 重构版）。
package com.tsloms.server;

import com.tsloms.server.config.AppProperties;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.EnableConfigurationProperties;

@SpringBootApplication
@EnableConfigurationProperties(AppProperties.class)
public class TslomsServerApplication {

    public static void main(String[] args) {
        SpringApplication.run(TslomsServerApplication.class, args);
    }
}
