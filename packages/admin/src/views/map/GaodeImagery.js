/**
 * 高德地图瓦片 ImageryProvider（用于 Cesium）
 *
 * 高德使用 GCJ-02 坐标系 + 标准 XYZ 瓦片（webrd 路网、wprd 影像）。
 * Cesium 为 WGS-84/WebMercator，本提供者按每个瓦片中心经纬度做
 * WGS-84 → GCJ-02 偏移后换算高德瓦片坐标，使高德实景贴合 Cesium 地球。
 */
import * as Cesium from 'cesium'

// GCJ-02 由 WGS-84 偏移（火星坐标近似算法）
const a = 6378245.0
const ee = 0.00669342162296594323

function outOfChina(lng, lat) {
  return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271
}
function transformLat(lng, lat) {
  let ret = -100.0 + 2.0 * lng + 3.0 * lat + 0.2 * lat * lat + 0.1 * lng * lat + 0.2 * Math.sqrt(Math.abs(lng))
  ret += (20.0 * Math.sin(6.0 * lng * Math.PI) + 20.0 * Math.sin(2.0 * lng * Math.PI)) * 2.0 / 3.0
  ret += (20.0 * Math.sin(lat * Math.PI) + 40.0 * Math.sin(lat / 3.0 * Math.PI)) * 2.0 / 3.0
  ret += (160.0 * Math.sin(lat / 12.0 * Math.PI) + 320 * Math.sin(lat * Math.PI / 30.0)) * 2.0 / 3.0
  return ret
}
function transformLng(lng, lat) {
  let ret = 300.0 + lng + 2.0 * lat + 0.1 * lng * lng + 0.1 * lng * lat + 0.1 * Math.sqrt(Math.abs(lng))
  ret += (20.0 * Math.sin(6.0 * lng * Math.PI) + 20.0 * Math.sin(2.0 * lng * Math.PI)) * 2.0 / 3.0
  ret += (20.0 * Math.sin(lng * Math.PI) + 40.0 * Math.sin(lng / 3.0 * Math.PI)) * 2.0 / 3.0
  ret += (150.0 * Math.sin(lng / 12.0 * Math.PI) + 300.0 * Math.sin(lng / 30.0 * Math.PI)) * 2.0 / 3.0
  return ret
}
function wgs84ToGcj02(lng, lat) {
  if (outOfChina(lng, lat)) return { lng, lat }
  let dLat = transformLat(lng - 105.0, lat - 35.0)
  let dLng = transformLng(lng - 105.0, lat - 35.0)
  const radLat = (lat / 180.0) * Math.PI
  let magic = Math.sin(radLat)
  magic = 1 - ee * magic * magic
  const sqrtMagic = Math.sqrt(magic)
  dLat = (dLat * 180.0) / ((a * (1 - ee)) / (magic * sqrtMagic) * Math.PI)
  dLng = (dLng * 180.0) / (a / sqrtMagic * Math.cos(radLat) * Math.PI)
  return { lng: lng + dLng, lat: lat + dLat }
}

function lngLatToTile(lng, lat, z) {
  const n = Math.pow(2, z)
  const x = ((lng + 180) / 360) * n
  const lngRad = (lat * Math.PI) / 180
  const y = ((1 - Math.log(Math.tan(lngRad) + 1 / Math.cos(lngRad)) / Math.PI) / 2) * n
  return { x: Math.floor(x), y: Math.floor(y) }
}

export default class GaodeImageryProvider {
  constructor(options = {}) {
    this._tilingScheme = new Cesium.WebMercatorTilingScheme({
      numberOfLevelZeroTilesX: 2,
      numberOfLevelZeroTilesY: 2,
    })
    this._rectangle = this._tilingScheme.rectangle
    this._tileWidth = 256
    this._tileHeight = 256
    this._minimumLevel = options.minimumLevel ?? 3
    this._maximumLevel = options.maximumLevel ?? 18
    this._style = options.style ?? 8 // 8=标准路网(带中文标注), 6=卫星影像
    this._credit = new Cesium.Credit('高德地图', true)
    this._errorEvent = new Cesium.Event()
    this._readyPromise = Promise.resolve(true)
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
    const rect = this._tilingScheme.tileXYToRectangle(x, y, level)
    const cLng = Cesium.Math.toDegrees((rect.west + rect.east) / 2)
    const cLat = Cesium.Math.toDegrees((rect.south + rect.north) / 2)
    const gc = wgs84ToGcj02(cLng, cLat)
    const bz = Math.min(level, this._maximumLevel)
    const t = lngLatToTile(gc.lng, gc.lat, bz)
    // 走同源代理（后端转发，避免高德对某些子域/浏览器的跨域限制，卫星更稳定）
    const url = `/tsloms/api/v1/proxy/gaode?x=${t.x}&y=${t.y}&z=${bz}&style=${this._style}`
    return Cesium.ImageryProvider.loadImage(this, url).catch(() => undefined)
  }
}
