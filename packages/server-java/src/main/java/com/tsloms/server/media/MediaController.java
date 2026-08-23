// 媒体接口：列表/手机上传(MIME嗅探)/监控流登记/删除，契约对齐 Go 版 handler/media.go。
package com.tsloms.server.media;

import com.tsloms.server.model.DeviceMedia;
import com.tsloms.server.model.OpTypes;
import com.tsloms.server.repository.DeviceMediaRepository;
import com.tsloms.server.repository.DeviceRepository;
import com.tsloms.server.web.ApiResponse;
import com.tsloms.server.web.AuthInterceptor;
import com.tsloms.server.web.OperationLogService;
import com.tsloms.server.web.Pagination;
import com.tsloms.server.web.RequirePerm;
import jakarta.servlet.http.HttpServletRequest;
import java.io.ByteArrayInputStream;
import java.net.URLConnection;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.time.Instant;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.data.jpa.domain.Specification;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.multipart.MultipartFile;

@RestController
public class MediaController {

    /** 存储目录（对齐 Go 版 MediaDir 默认）。 */
    public static final String MEDIA_DIR = "./uploads/media";
    public static final String URL_PREFIX = "/media";

    private static final DateTimeFormatter MONTH_FMT =
            DateTimeFormatter.ofPattern("yyyyMM").withZone(ZoneId.systemDefault());
    private static final List<String> ALLOWED_EXTS = List.of(
            ".jpg", ".jpeg", ".png", ".gif", ".mp4", ".mov", ".webm", ".avi",
            ".pdf", ".doc", ".docx");
    private static final long MAX_SIZE = 200L * 1024 * 1024;

    private final DeviceMediaRepository mediaRepo;
    private final DeviceRepository devices;
    private final OperationLogService opLog;

    public MediaController(DeviceMediaRepository mediaRepo, DeviceRepository devices,
                           OperationLogService opLog) {
        this.mediaRepo = mediaRepo;
        this.devices = devices;
        this.opLog = opLog;
    }

    // ---------------- 工具（对齐 Go 版同名函数） ----------------

    static String categoryOf(String ext) {
        return switch (ext) {
            case ".jpg", ".jpeg", ".png", ".gif" -> DeviceMedia.CATEGORY_PHOTO;
            case ".pdf", ".doc", ".docx" -> DeviceMedia.CATEGORY_DOC;
            default -> DeviceMedia.CATEGORY_VIDEO;
        };
    }

    static String thumbOf(String ext, String url) {
        return DeviceMedia.CATEGORY_PHOTO.equals(categoryOf(ext)) ? url : "";
    }

    /** 校验流地址协议：rtsp/rtsps/http/https 且带 host。 */
    static boolean validStreamURL(String s) {
        if (s == null) {
            return false;
        }
        String low = s.trim().toLowerCase(Locale.ROOT);
        boolean schemeOk = low.startsWith("rtsp://") || low.startsWith("rtsps://")
                || low.startsWith("http://") || low.startsWith("https://");
        if (!schemeOk) {
            return false;
        }
        try {
            var uri = java.net.URI.create(s.trim());
            return uri.getHost() != null && !uri.getHost().isEmpty();
        } catch (Exception e) {
            return false;
        }
    }

    /** 清洗硬件 ID：仅保留字母数字，杜绝路径穿越注入。 */
    static String sanitizeHwID(String s) {
        StringBuilder b = new StringBuilder();
        for (char ch : s == null ? new char[0] : s.toCharArray()) {
            if ((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
                b.append(ch);
            }
        }
        return b.toString();
    }

    /** MIME 与扩展名一致性校验（图片 image/*；视频宽松放行容器；文档按 magic）。 */
    static boolean mimeAllowed(String ext, String detected) {
        if (detected == null || detected.isEmpty()) {
            return true;
        }
        boolean isImg = List.of(".jpg", ".jpeg", ".png", ".gif").contains(ext);
        boolean isVid = List.of(".mp4", ".mov", ".webm", ".avi").contains(ext);
        boolean isDoc = List.of(".pdf", ".doc", ".docx").contains(ext);
        if (isImg) {
            return detected.startsWith("image/");
        }
        if (isVid) {
            return detected.startsWith("video/") || "application/octet-stream".equals(detected)
                    || "application/mp4".equals(detected) || "video/mp4".equals(detected);
        }
        if (isDoc) {
            return "application/pdf".equals(detected)
                    || "application/octet-stream".equals(detected)
                    || "application/zip".equals(detected)
                    || "application/msword".equals(detected)
                    || detected.contains("officedocument")
                    || "application/x-ole-storage".equals(detected);
        }
        return false;
    }

    /**
     * 前 512 字节 MIME 嗅探。
     * 对齐 Go 版 http.DetectContentType 关键行为：未知字节流要么给出具体类型，
     * 要么归为 text/plain（可打印）或 application/octet-stream，绝不能返回空导致放行。
     */
    static String detectMime(byte[] b) {
        if (b == null || b.length == 0) {
            return "";
        }
        // 图片 magic
        if (b.length > 8 && (b[0] & 0xFF) == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G') {
            return "image/png";
        }
        if (b.length > 3 && (b[0] & 0xFF) == 0xFF && (b[1] & 0xFF) == 0xD8
                && (b[2] & 0xFF) == 0xFF) {
            return "image/jpeg";
        }
        if (b.length > 6 && b[0] == 'G' && b[1] == 'I' && b[2] == 'F' && b[3] == '8') {
            return "image/gif";
        }
        if (b.length > 5 && b[0] == '%' && b[1] == 'P' && b[2] == 'D' && b[3] == 'F') {
            return "application/pdf";
        }
        if (b.length > 4 && b[0] == 'P' && b[1] == 'K' && (b[2] & 0xFF) == 3 && (b[3] & 0xFF) == 4) {
            return "application/zip"; // docx 容器
        }
        // MP4：偏移 4-7 为 ftyp
        if (b.length > 8 && b[4] == 'f' && b[5] == 't' && b[6] == 'y' && b[7] == 'p') {
            return "video/mp4";
        }
        // 可打印文本判定（含常见空白）
        int printable = 0;
        for (byte x : b) {
            char c = (char) (x & 0xFF);
            boolean ok = c == '\t' || c == '\n' || c == '\r' || c == 0x0C
                    || (c >= 0x20 && c != 0x7F);
            if (ok) {
                printable++;
            }
        }
        return printable * 100 / b.length >= 80
                ? "text/plain; charset=utf-8"
                : "application/octet-stream";
    }

    // ---------------- 接口 ----------------

    /** GET /media：分页 + 设备/类型/来源筛选。 */
    @GetMapping("/api/v1/media")
    public ApiResponse<Map<String, Object>> list(
            @RequestParam(name = "device_hw_id", required = false) String deviceHwId,
            @RequestParam(name = "media_type", required = false) String mediaType,
            @RequestParam(required = false) String source,
            HttpServletRequest request) {
        Pagination.Page pg = Pagination.of(request);

        Specification<DeviceMedia> spec = (root, query, cb) -> {
            var preds = new ArrayList<jakarta.persistence.criteria.Predicate>();
            if (deviceHwId != null && !deviceHwId.isBlank()) {
                preds.add(cb.equal(root.get("deviceHwId"), deviceHwId));
            }
            if (mediaType != null && !mediaType.isBlank()) {
                preds.add(cb.equal(root.get("mediaType"), mediaType));
            }
            if (source != null && !source.isBlank()) {
                preds.add(cb.equal(root.get("source"), source));
            }
            return cb.and(preds.toArray(new jakarta.persistence.criteria.Predicate[0]));
        };

        long total = mediaRepo.count(spec);
        List<DeviceMedia> rows = mediaRepo.findAll(spec,
                PageRequest.of(pg.page() - 1, pg.pageSize(),
                        Sort.by(Sort.Direction.DESC, "createdAt")))
                .getContent();
        Map<String, Object> data = new LinkedHashMap<>();
        data.put("list", rows);
        data.put("total", total);
        data.put("page", pg.page());
        data.put("page_size", pg.pageSize());
        return ApiResponse.ok(data);
    }

    /** POST /media/upload（media:upload）：举证/图片上传，MIME 嗅探防伪装。 */
    @PostMapping("/api/v1/media/upload")
    @RequirePerm("media:upload")
    public ResponseEntity<?> upload(@RequestParam(name = "device_hw_id") String hwId,
                                    @RequestParam(name = "media_type", required = false) String mediaType,
                                    @RequestParam(required = false) String title,
                                    @RequestParam(required = false) String note,
                                    @RequestParam(required = false) String intersection,
                                    @RequestParam(name = "light_color", required = false) String lightColor,
                                    @RequestParam(name = "fault_desc", required = false) String faultDesc,
                                    @RequestParam(name = "is_active_fault", required = false) String isActiveFault,
                                    @RequestParam("file") MultipartFile file,
                                    HttpServletRequest request) throws Exception {
        String type = (mediaType == null || mediaType.isBlank())
                ? DeviceMedia.TYPE_EVIDENCE : mediaType;

        // 举证必须填路口（便于定位与派单）
        String inter = intersection == null ? "" : intersection.trim();
        if (DeviceMedia.TYPE_EVIDENCE.equals(type) && inter.isEmpty()) {
            return badRequest("请填写路口名称（便于定位与派单）");
        }

        String safeHw = sanitizeHwID(hwId);
        if (safeHw.isEmpty()) {
            return badRequest("设备硬件ID不合法");
        }
        if (file == null || file.isEmpty()) {
            return badRequest("请选择上传文件");
        }
        String original = file.getOriginalFilename() == null ? "" : file.getOriginalFilename();
        int dot = original.lastIndexOf('.');
        String ext = dot < 0 ? "" : original.substring(dot).toLowerCase(Locale.ROOT);
        if (!ALLOWED_EXTS.contains(ext)) {
            return badRequest("不支持的文件类型（jpg/png/gif/mp4/mov/webm/avi）");
        }
        if (file.getSize() > MAX_SIZE) {
            return badRequest("文件过大（最大200MB）");
        }

        // MIME 嗅探（前 512 字节），拒绝伪装文件
        byte[] head = file.getInputStream().readNBytes(512);
        if (head.length > 0 && !mimeAllowed(ext, detectMime(head))) {
            return badRequest("文件内容与扩展名不符，已拒绝上传");
        }

        // 存储：{MEDIA_DIR}/{yyyyMM}/{hwSafe}_{ts}{ext}
        Path dir = Path.of(MEDIA_DIR, MONTH_FMT.format(Instant.now()));
        Files.createDirectories(dir);
        String fname = safeHw + "_" + System.currentTimeMillis() + ext;
        Path target = dir.resolve(fname);
        Files.copy(file.getInputStream(), target, StandardCopyOption.REPLACE_EXISTING);
        String rel = URL_PREFIX + "/" + MONTH_FMT.format(Instant.now()) + "/" + fname;

        DeviceMedia m = new DeviceMedia();
        m.deviceHwId = safeHw;
        m.mediaType = type;
        m.category = categoryOf(ext);
        m.title = title == null ? "" : title;
        m.source = DeviceMedia.SOURCE_UPLOAD;
        m.url = rel;
        m.thumbnail = thumbOf(ext, rel);
        m.note = note == null ? "" : note;
        Object uname = request.getAttribute(AuthInterceptor.ATTR_USERNAME);
        m.uploadedBy = uname == null ? "" : String.valueOf(uname);
        m.intersection = inter;
        m.lightColor = lightColor == null ? "" : lightColor;
        m.faultDesc = faultDesc == null ? "" : faultDesc;
        m.isActiveFault = Boolean.parseBoolean(isActiveFault);
        try {
            mediaRepo.save(m);
        } catch (Exception e) {
            Files.deleteIfExists(target); // 落库失败回滚文件
            throw e;
        }
        opLog.record(request, OpTypes.CREATE, "media/" + m.id, "上传设备媒体");
        return ok(Map.of("media", m, "message", "上传成功"));
    }

    /** 监控流登记请求体（对齐 Go 版 CreateRTSPMedia req）。 */
    public record StreamRequest(String deviceHwId, String mediaType, String title,
                                String url, String compatibleUrl, String thumbnail,
                                Integer duration, String note, String intersection) {
    }

    /** POST /media/streams（media:upload + 视频模块启用）：登记 RTSP/云 URL 流。 */
    @PostMapping("/api/v1/media/streams")
    @RequirePerm("media:upload")
    public ResponseEntity<?> createStream(@RequestBody StreamRequest req,
                                          HttpServletRequest request) {
        if (req.deviceHwId() == null || req.deviceHwId().isBlank()
                || req.mediaType() == null || req.mediaType().isBlank()
                || req.url() == null || req.url().isBlank()) {
            return badRequest("参数错误（device_hw_id、media_type、url 必填）");
        }
        // 类型白名单：登记仅限监控/时间视频；举证走上传
        if (!DeviceMedia.TYPE_MONITORING.equals(req.mediaType())
                && !DeviceMedia.TYPE_TIMELAPSE.equals(req.mediaType())) {
            return badRequest("media_type 不合法：仅支持 monitoring / timelapse（举证请使用上传）");
        }
        if (!validStreamURL(req.url())) {
            return badRequest("视频地址不合法：需以 rtsp://、rtsps://、http:// 或 https:// 开头");
        }
        if (req.compatibleUrl() != null && !req.compatibleUrl().isEmpty()
                && !validStreamURL(req.compatibleUrl())) {
            return badRequest("兼容播放地址不合法：需为 http(s)://、rtsp(s):// 或 HLS/FLV 地址");
        }
        if (devices.findByHwId(req.deviceHwId()).isEmpty()) {
            return badRequest("关联设备不存在");
        }

        boolean isRtsp = req.url().toLowerCase(Locale.ROOT).startsWith("rtsp://")
                || req.url().toLowerCase(Locale.ROOT).startsWith("rtsps://");
        String warning = "";
        if (isRtsp && (req.compatibleUrl() == null || req.compatibleUrl().isEmpty())) {
            warning = "RTSP 源无法在浏览器直接播放，建议补充兼容播放地址(HLS/FLV)以便监控大屏同屏直播";
        }

        DeviceMedia m = new DeviceMedia();
        m.deviceHwId = req.deviceHwId();
        m.mediaType = req.mediaType();
        m.category = DeviceMedia.CATEGORY_VIDEO;
        m.title = req.title() == null ? "" : req.title();
        m.source = isRtsp ? DeviceMedia.SOURCE_RTSP : DeviceMedia.SOURCE_URL;
        m.url = req.url();
        m.compatibleUrl = req.compatibleUrl() == null ? "" : req.compatibleUrl();
        m.thumbnail = req.thumbnail() == null ? "" : req.thumbnail();
        m.duration = req.duration() == null ? 0 : req.duration();
        m.note = req.note() == null ? "" : req.note();
        m.intersection = req.intersection() == null ? "" : req.intersection();
        mediaRepo.save(m);

        opLog.record(request, OpTypes.CREATE, "media/" + m.id, "登记监控/视频流");
        Map<String, Object> resp = new LinkedHashMap<>();
        resp.put("media", m);
        resp.put("message", "登记成功");
        if (!warning.isEmpty()) {
            resp.put("warning", warning);
        }
        return ResponseEntity.ok(ApiResponse.ok(resp));
    }

    /** DELETE /media/{id}（media:delete）：删除记录并清理本地文件。 */
    @DeleteMapping("/api/v1/media/{id}")
    @RequirePerm("media:delete")
    public ResponseEntity<?> delete(@PathVariable Long id, HttpServletRequest request)
            throws Exception {
        var opt = mediaRepo.findById(id);
        if (opt.isEmpty()) {
            return notFound("媒体不存在");
        }
        DeviceMedia m = opt.get();
        mediaRepo.delete(m);
        if (DeviceMedia.SOURCE_UPLOAD.equals(m.source) && m.url != null) {
            Path local = Path.of(".", m.url.substring(URL_PREFIX.length()));
            Files.deleteIfExists(local);
        }
        opLog.record(request, OpTypes.DELETE, "media/" + id, "删除设备媒体");
        return ok(Map.of("message", "删除成功"));
    }

    private ResponseEntity<?> badRequest(String msg) {
        return ResponseEntity.badRequest().body(ApiResponse.fail("bad_request", msg));
    }

    private ResponseEntity<?> notFound(String msg) {
        return ResponseEntity.status(404).body(ApiResponse.fail("not_found", msg));
    }

    private ResponseEntity<?> ok(Map<String, Object> data) {
        return ResponseEntity.ok(ApiResponse.ok(data));
    }
}
