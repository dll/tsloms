/** 构建版本信息：主次版本由 package.json 维护，补丁号由流水线构建号生成。 */
export const APP_VERSION = import.meta.env.VITE_APP_VERSION || '0.1.0'
export const BUILD_COMMIT = import.meta.env.VITE_BUILD_COMMIT || 'local'
export const BUILD_NUMBER = import.meta.env.VITE_BUILD_NUMBER || 'local'
export const BUILD_TIME = import.meta.env.VITE_BUILD_TIME || ''
