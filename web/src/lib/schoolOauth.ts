export const HENAU_WECHAT_OAUTH_APPID = 'wx312828c5c278c93c'
export const HENAU_WECHAT_OAUTH_REDIRECT_URI = 'https://xhbcs.henau.edu.cn'
export const HENAU_WECHAT_CALLBACK_EXAMPLE =
  'https://xhbcs.henau.edu.cn/?code=001B8Zfa1NMRHL0m65la1gbfBa3B8ZFy&state=STATE#/checkin'

export type SchoolOauthInputKind = 'empty' | 'code' | 'callback-url' | 'invalid'

export interface SchoolOauthInputDetection {
  kind: SchoolOauthInputKind
  code: string
}

export function createWechatOauthState(): string {
  const rand = Math.random().toString(36).slice(2, 10)
  return `wangui-${Date.now().toString(36)}-${rand}`
}

export function buildWechatOauthAuthorizeUrl(state: string): string {
  const url = new URL('https://open.weixin.qq.com/connect/oauth2/authorize')
  url.searchParams.set('appid', HENAU_WECHAT_OAUTH_APPID)
  url.searchParams.set('redirect_uri', HENAU_WECHAT_OAUTH_REDIRECT_URI)
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('scope', 'snsapi_userinfo')
  url.searchParams.set('state', state || 'STATE')
  url.searchParams.set('connect_redirect', '1')
  return url.toString() + '#wechat_redirect'
}

export function detectSchoolOauthInput(raw: string): SchoolOauthInputDetection {
  const value = raw.trim()
  if (!value) return { kind: 'empty', code: '' }
  if (isLikelyRawCode(value)) return { kind: 'code', code: value }

  const code = extractOAuthCodeFromInput(value)
  if (code) return { kind: 'callback-url', code }
  return { kind: 'invalid', code: '' }
}

function isLikelyRawCode(value: string): boolean {
  return value.length > 0 && !/[?#=&/\s]/.test(value)
}

function extractOAuthCodeFromInput(raw: string): string {
  const value = raw.trim()
  if (!value) return ''

  const fromUrl = codeFromUrl(value)
  if (fromUrl) return fromUrl

  if (value.startsWith('?')) {
    return new URLSearchParams(value.slice(1)).get('code')?.trim() || ''
  }

  if (value.includes('code=')) {
    return new URLSearchParams(value).get('code')?.trim() || ''
  }

  return ''
}

function codeFromUrl(value: string): string {
  try {
    const url = new URL(value)
    const direct = url.searchParams.get('code')?.trim()
    if (direct) return direct

    const fragment = url.hash.replace(/^#/, '')
    const queryIndex = fragment.indexOf('?')
    if (queryIndex >= 0) {
      return new URLSearchParams(fragment.slice(queryIndex + 1)).get('code')?.trim() || ''
    }
  } catch {
    return ''
  }
  return ''
}
