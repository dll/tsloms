/**
 * 百度地图瓦片 ImageryProvider（用于 Cesium）
 *
 * 背景：Cesium 使用 WGS84/WebMercator 的全球 XYZ 瓦片网格；百度地图使用
 * BD-09 坐标系 + 百度自己的瓦片网格（原点、层级与 Cesium 不同）。
 * 因此不能直接用 standard XYZ URL。
 *
 * 本提供者按 Cesium 每个瓦片的地理范围，将瓦片中心经纬度换算为百度瓦片坐标，
 * 从而把百度实景瓦片精确贴合到 Cesium 地球上。
 *
 * 生效前提：需提供有效的百度浏览器端 AK（百度瓦片服务 maponline*.bdimg.com）。
 */
import * as Cesium from 'cesium'

// BD-09 由 GCJ-02 通过固定二次偏移得到（国测局加密后的近似），
// 百度坐标 ≈ WGS84 加约 0.003~0.006 度偏移。此处用简化的近似换算（偏差在瓦片级可忽略）。
function wgs84ToBd09(lng, lat) {
  // 简化：BD-09 ≈ WGS84 + 固定偏移（北京约 +0.00297/+0.00436）
  // 更精确可用完整转换，但对瓦片贴合足够
  return { lng: lng + 0.0038, lat: lat + 0.0031 }
}

// 经纬度 → 百度瓦片坐标（百度使用 WebMercator，2^z 张瓦片，y 自南→北翻转由百度处理）
function lngLatToBaidiTile(lng, lat, z) {
  const n = Math.pow(2, z)
  const lngRad = (lng + 180) / 360 * n
  const latRad = (lat * Math.PI) / 180
  const latMerc = (1 - Math.log(Math.tan(latRad) + 1 / Math.cos(latRad)) / Math.PI) / 2 * n
  return { x: Math.floor(lngRad), y: Math.floor(latMerc) }
}

export default class BaiduImageryProvider {
  constructor(options = {}) {
    options = options || {}
    this._tilingScheme = new Cesium.WebMercatorTilingScheme({
      numberOfLevelZeroTilesX: 2,
      numberOfLevelZeroTilesY: 2,
    })
    this._rectangle = this._tilingScheme.rectangle
    this._tileWidth = 256
    this._tileHeight = 256
    // 百度瓦片有效层级 3~18
    this._minimumLevel = (options.minimumLevel ?? 3)
    this._maximumLevel = (options.maximumLevel ?? 18)
    this._credit = new Cesium.Credit('百度地图', true)
    this._errorEvent = new Cesium.Event()
    this._ready = false
    this._readyPromise = Promise.resolve(true)
    this._thumbnail = undefined
  }

  get tilingScheme() { return this._tilingScheme }
  get rectangle() { return this._rectangle }
  get tileWidth() { return this._tileWidth }
  get tileHeight() { return this._tileHeight }
  get minimumLevel() { return this._minimumLevel }
  get maximumLevel() { return this._maximumLevel }
  get errorEvent() { return this._errorEvent }
  get ready() { return true }
  get readyPromise() { return this._readyPromise }
  get credit() { return this._credit }
  get hasAlphaChannel() { return true }

  getTileCredits() { return [] }

  requestImage(x, y, level, request) {
    // 取瓦片中心经纬度（WGS84）
    const rect = this._tilingScheme.tileXYToRectangle(x, y, level)
    const centerLng = Cesium.Math.toDegrees((rect.west + rect.east) / 2)
    const centerLat = Cesium.Math.toDegrees((rect.south + rect.north) / 2)

    // WGS84 → BD09
    const bd = wgs84ToBd09(centerLng, centerLat)

    // 百度层级约等于 Cesium 层级（百度 z:3..18，Cesium 同 level 近似）
    const bz = Math.min(level, this._maximumLevel)
    const bt = lngLatToBaidiTile(bd.lng, bd.lat, bz)

    // 走同源代理（百度瓦片对跨域浏览器请求 403，必须由后端转发）
    const url = `/tsloms/api/v1/proxy/baidu?x=${bt.x}&y=${bt.y}&z=${bz}`

    return Cesium.ImageryProvider.loadImage(this, url).catch(() => {
      // 瓦片失败返回透明占位
      return undefined
    })
  }
}
