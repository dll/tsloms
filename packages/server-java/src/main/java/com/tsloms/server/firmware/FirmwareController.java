// 固件接口：列表/详情/上传(multipart)/更新/发布/删除 + 升级记录管理。
// 契约对齐 Go 版 handler/firmware.go（版本解析、MD5 校验、50MB 上限、扩展名白名单）。
package com.tsloms.server.firmware;

import com.tsloms.server.model.FirmwarePackage;
import com.tsloms.server.model.FirmwareUpgradeRecord;
import com.tsloms.server.model.FirmwareUpgradeStatuses;
import com.tsloms.server.model.OpTypes;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.repository.FirmwarePackageRepository;
import com.tsloms.server.repository.FirmwareUpgradeRecordRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import jakarta.servlet.http.HttpServletRequest;
import java.io.InputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.regex.Pattern;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.multipart.MultipartFile;

@RestController
public class FirmwareController {

    /** 固件存储目录（MEDIA_DIR 默认 ./uploads/media，与 Go 版一致）。 */
    public static final String MEDIA_DIR = "./uploads/media";

    private static final Pattern VERSION_REGEX =
            Pattern.compile("^[vV]?(\\d+)(?:\\.(\\d+))?(?:\\.(\\d+))?$");
    private static final List<String> FW_EXTS =
            List.of(".bin", ".hex", ".fw", ".elf", ".img", ".dat");
    private static final long MAX_SIZE = 50L * 1024 * 1024;

    private final FirmwarePackageRepository pkgs;
    private final FirmwareUpgradeRecordRepository upgrades;
    private final DeviceRepository devices;
    private final OperationLogService opLog;

    public FirmwareController(FirmwarePackageRepository pkgs,
                              FirmwareUpgradeRecordRepository upgrades,
                              DeviceRepository devices, OperationLogService opLog) {
        this.pkgs = pkgs;
        this.upgrades = upgrades;
        this.devices = devices;
        this.opLog = opLog;
    }

    // ---------------- 版本解析（对齐 Go 版 parseVersion） ----------------

    record Ver(long major, long minor, long build) {
    }

    static Ver parseVersion(String v) {
        var m = VERSION_REGEX.matcher(v == null ? "" : v.trim());
        if (!m.matches()) {
            throw new IllegalArgumentException("版本号格式不合法，需形如 v1.2.3 / 1.2.3");
        }
        return new Ver(num(m.group(1)), num(m.group(2)), num(m.group(3)));
    }

    private static long num(String s) {
        return s == null ? 0 : Long.parseLong(s);
    }

    static boolean isFirmwareExt(String ext) {
        return FW_EXTS.contains(ext.toLowerCase());
    }

    // ---------------- 视图 ----------------

    private Map<String, Object> view(FirmwarePackage f) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", f.id);
        m.put("version", f.version);
        m.put("major", f.major);
        m.put("minor", f.minor);
        m.put("build", f.build);
        m.put("sw_version", f.swVersion);
        m.put("file_name", f.fileName);
        m.put("file_path", f.filePath);
        m.put("size", f.size);
        m.put("md5", f.md5);
        m.put("description", f.description);
        m.put("published", f.published);
        m.put("published_at", f.publishedAt);
        m.put("uploader", f.uploader);
        m.put("created_at", f.createdAt);
        return m;
    }

    // ---------------- CRUD ----------------

    /** GET /firmwares：分页，published=true/1 过滤已发布。 */
    @GetMapping("/api/v1/firmwares")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(required = false) String published,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);
        boolean onlyPublished = "true".equals(published) || "1".equals(published);

        var pageData = pkgs.findAll(
                onlyPublished ? com.tsloms.server.repository.FirmwarePackageRepository.PUBLISHED
                        : com.tsloms.server.repository.FirmwarePackageRepository.ALL,
                PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt")));
        List<Object> items = new ArrayList<>();
        pageData.forEach(f -> items.add(view(f)));
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", items);
        data.put("total", pageData.getTotalElements());
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** GET /firmwares/{id}。 */
    @GetMapping("/api/v1/firmwares/{id}")
    public ResponseEntity<?> get(@PathVariable Long id) {
        var opt = pkgs.findById(id);
        if (opt.isEmpty()) {
            return notFound("固件包不存在");
        }
        return ok(Map.of("firmware", view(opt.get())));
    }

    /** POST /firmwares/upload（firmware:manage）：multipart version/description/file。 */
    @PostMapping("/api/v1/firmwares/upload")
    @com.tsloms.server.web.RequirePerm("firmware:manage")
    public ResponseEntity<?> upload(@RequestParam("version") String version,
                                    @RequestParam(value = "description", required = false) String description,
                                    @RequestParam("file") MultipartFile file,
                                    HttpServletRequest request) throws Exception {
        String ver = version == null ? "" : version.trim();
        if (ver.isEmpty()) {
            return badRequest("请填写固件版本号");
        }
        Ver v;
        try {
            v = parseVersion(ver);
        } catch (IllegalArgumentException e) {
            return badRequest(e.getMessage());
        }
        if (file == null || file.isEmpty()) {
            return badRequest("请选择固件文件");
        }
        String original = file.getOriginalFilename() == null ? "" : file.getOriginalFilename();
        int dot = original.lastIndexOf('.');
        String ext = dot < 0 ? "" : original.substring(dot).toLowerCase();
        if (!isFirmwareExt(ext)) {
            return badRequest("固件文件类型不支持（bin/hex/fw/elf 等二进制）");
        }
        if (file.getSize() > MAX_SIZE) {
            return badRequest("固件文件过大（最大50MB）");
        }
        if (pkgs.existsByVersion(version)) {
            return badRequest("该固件版本已存在");
        }

        Path dir = Path.of(MEDIA_DIR, "firmware");
        Files.createDirectories(dir);
        String ts = java.time.format.DateTimeFormatter.ofPattern("yyyyMMddHHmmss")
                .withZone(java.time.ZoneId.systemDefault()).format(Instant.now());
        String safeVer = ver.replaceAll("[^0-9A-Za-z._-]", "_");
        String fname = "fw_" + safeVer + "_" + ts + ext;
        Path target = dir.resolve(fname).normalize();
        if (!target.toAbsolutePath().normalize()
                .startsWith(dir.toAbsolutePath().normalize())) {
            return badRequest("非法路径");
        }
        try (InputStream in = file.getInputStream()) {
            Files.copy(in, target, StandardCopyOption.REPLACE_EXISTING);
        }
        String md5 = md5OfFile(target);
        String rel = "firmware/" + fname;

        Object opUser = request.getAttribute(AuthInterceptor.ATTR_USERNAME);
        String uploader = opUser == null || String.valueOf(opUser).isEmpty()
                ? "system" : String.valueOf(opUser);

        FirmwarePackage pkg = new FirmwarePackage();
        pkg.version = version;
        pkg.major = v.major();
        pkg.minor = v.minor();
        pkg.build = v.build();
        pkg.swVersion = (v.major() << 28) | (v.minor() << 24);
        pkg.fileName = original;
        pkg.filePath = rel;
        pkg.size = file.getSize();
        pkg.md5 = md5;
        pkg.description = description == null ? "" : description;
        pkg.published = false;
        pkg.uploader = uploader;
        try {
            pkgs.save(pkg);
        } catch (Exception e) {
            Files.deleteIfExists(target); // 落库失败回滚文件
            return serverError();
        }
        opLog.record(request, OpTypes.CREATE, "firmware/" + pkg.id, "上传固件包 " + pkg.version);
        return ok(Map.of("firmware", view(pkg), "url", "/media/firmware/" + fname));
    }

    /** 更新请求体。 */
    public record UpdateRequest(String description, String version) {
    }

    /** PUT /firmwares/{id}：更新描述/改版本号（唯一性+位域重算）。 */
    @PutMapping("/api/v1/firmwares/{id}")
    @com.tsloms.server.web.RequirePerm("firmware:manage")
    public ResponseEntity<?> update(@PathVariable Long id, @RequestBody UpdateRequest req,
                                    HttpServletRequest request) {
        var opt = pkgs.findById(id);
        if (opt.isEmpty()) {
            return notFound("固件包不存在");
        }
        FirmwarePackage f = opt.get();
        if (req.description() != null && !req.description().isEmpty()) {
            f.description = req.description();
        }
        if (req.version() != null && !req.version().isEmpty()
                && !req.version().equals(f.version)) {
            Ver v;
            try {
                v = parseVersion(req.version());
            } catch (IllegalArgumentException e) {
                return badRequest(e.getMessage());
            }
            if (pkgs.existsByVersion(req.version())) {
                return badRequest("该固件版本已存在");
            }
            f.version = req.version();
            f.major = v.major();
            f.minor = v.minor();
            f.build = v.build();
            f.swVersion = (v.major() << 28) | (v.minor() << 24);
        }
        pkgs.save(f);
        opLog.record(request, OpTypes.UPDATE, "firmware/" + id, "更新固件包 " + f.version);
        return ok(Map.of("message", "更新成功"));
    }

    /** 发布请求体。 */
    public record PublishRequest(boolean published) {
    }

    /** PUT /firmwares/{id}/publish：发布/下线。 */
    @PutMapping("/api/v1/firmwares/{id}/publish")
    @com.tsloms.server.web.RequirePerm("firmware:manage")
    public ResponseEntity<?> publish(@PathVariable Long id, @RequestBody PublishRequest req,
                                     HttpServletRequest request) {
        var opt = pkgs.findById(id);
        if (opt.isEmpty()) {
            return notFound("固件包不存在");
        }
        FirmwarePackage f = opt.get();
        f.published = req.published();
        f.publishedAt = req.published() ? Instant.now() : null;
        pkgs.save(f);
        String action = req.published() ? "发布固件" : "下线固件";
        opLog.record(request, OpTypes.UPDATE, "firmware/" + id, action + " " + f.version);
        return ok(Map.of("message", action + "成功"));
    }

    /** DELETE /firmwares/{id}（firmware:delete）：删库并尝试清理本地文件。 */
    @DeleteMapping("/api/v1/firmwares/{id}")
    @com.tsloms.server.web.RequirePerm("firmware:delete")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request) throws Exception {
        var opt = pkgs.findById(id);
        if (opt.isEmpty()) {
            return notFound("固件包不存在");
        }
        FirmwarePackage f = opt.get();
        pkgs.delete(f);
        if (f.filePath != null && !f.filePath.isEmpty()) {
            String base = Path.of(f.filePath).getFileName().toString();
            Files.deleteIfExists(Path.of(MEDIA_DIR, "firmware", base));
        }
        opLog.record(request, OpTypes.DELETE, "firmware/" + id, "删除固件包 " + f.version);
        return ok(Map.of("message", "删除成功"));
    }

    // ---------------- 升级记录 ----------------

    /** GET /firmware-upgrades：分页 + 设备/状态筛选。 */
    @GetMapping("/api/v1/firmware-upgrades")
    public ApiResponse<Map<String, Object>> listUpgrades(
            @RequestParam(name = "device_hw_id", required = false) String deviceHwId,
            @RequestParam(required = false) String status,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);
        List<FirmwareUpgradeRecord> rows = upgrades.findAll(
                PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt"))).getContent();
        List<Object> items = new ArrayList<>();
        for (var r : rows) {
            if (deviceHwId != null && !deviceHwId.isBlank()
                    && !deviceHwId.equals(r.deviceHwId)) {
                continue;
            }
            if (status != null && !status.isBlank() && !status.equals(r.status)) {
                continue;
            }
            items.add(upgradeView(r));
        }
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", items);
        data.put("total", items.size());
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    private Map<String, Object> upgradeView(FirmwareUpgradeRecord r) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("id", r.id);
        m.put("firmware_id", r.firmwareId);
        m.put("device_hw_id", r.deviceHwId);
        m.put("target_version", r.targetVersion);
        m.put("status", r.status);
        m.put("error_msg", r.errorMsg);
        m.put("started_at", r.startedAt);
        m.put("finished_at", r.finishedAt);
        m.put("created_at", r.createdAt);
        return m;
    }

    /** 升级创建请求体。 */
    public record CreateUpgradeRequest(String deviceHwId, Long firmwareId) {
    }

    /** POST /firmware-upgrades（firmware:manage）：发起升级任务。 */
    @PostMapping("/api/v1/firmware-upgrades")
    @com.tsloms.server.web.RequirePerm("firmware:manage")
    public ResponseEntity<?> createUpgrade(@RequestBody CreateUpgradeRequest req,
                                           HttpServletRequest request) {
        if (req.deviceHwId() == null || req.deviceHwId().isBlank()
                || req.firmwareId() == null) {
            return badRequest("参数错误（device_hw_id、firmware_id 必填）");
        }
        if (devices.findByHwId(req.deviceHwId()).isEmpty()) {
            return badRequest("关联设备不存在");
        }
        var fwOpt = pkgs.findById(req.firmwareId());
        if (fwOpt.isEmpty()) {
            return notFound("固件包不存在");
        }
        FirmwarePackage fw = fwOpt.get();
        if (!fw.published) {
            return badRequest("固件未发布，无法发起升级");
        }
        boolean hasOpenTask = upgrades.findAll().stream()
                .anyMatch(r -> req.deviceHwId().equals(r.deviceHwId)
                        && (FirmwareUpgradeStatuses.PENDING.equals(r.status)
                                || FirmwareUpgradeStatuses.UPGRADING.equals(r.status)));
        if (hasOpenTask) {
            return badRequest("该设备已有未完成的升级任务");
        }
        Instant now = Instant.now();
        FirmwareUpgradeRecord rec = new FirmwareUpgradeRecord();
        rec.firmwareId = req.firmwareId();
        rec.deviceHwId = req.deviceHwId();
        rec.targetVersion = fw.version;
        rec.status = FirmwareUpgradeStatuses.PENDING;
        rec.startedAt = now;
        upgrades.save(rec);
        opLog.record(request, OpTypes.CREATE, "firmware-upgrade/" + rec.id,
                "发起设备 " + req.deviceHwId() + " 固件升级到 " + fw.version);
        return ok(Map.of("record", rec.id, "message", "升级任务已创建"));
    }

    /** DELETE /firmware-upgrades/{id}（firmware:delete）。 */
    @DeleteMapping("/api/v1/firmware-upgrades/{id}")
    @com.tsloms.server.web.RequirePerm("firmware:delete")
    public ResponseEntity<?> deleteUpgrade(@PathVariable Long id, HttpServletRequest request) {
        var opt = upgrades.findById(id);
        if (opt.isEmpty()) {
            return notFound("升级记录不存在");
        }
        upgrades.delete(opt.get());
        opLog.record(request, OpTypes.DELETE, "firmware-upgrade/" + id, "删除升级记录");
        return ok(Map.of("message", "删除成功"));
    }

    // ------------------------------------------------------------------

    private static String md5OfFile(Path path) throws Exception {
        MessageDigest md = MessageDigest.getInstance("MD5");
        try (InputStream in = Files.newInputStream(path)) {
            byte[] buf = new byte[8192];
            int n;
            while ((n = in.read(buf)) > 0) {
                md.update(buf, 0, n);
            }
        }
        return HexFormat.of().formatHex(md.digest());
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private ResponseEntity<?> notFound(String msg) {
        return ResponseEntity.status(404).body(ApiResponse.fail("not_found", msg));
    }

    private ResponseEntity<?> serverError() {
        return ResponseEntity.internalServerError()
                .body(ApiResponse.fail("internal_error", "服务器内部错误"));
    }

    private ResponseEntity<?> ok(Map<String, Object> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}
