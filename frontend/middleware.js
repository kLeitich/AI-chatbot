import { NextResponse } from 'next/server'

export const originCache = new Map()
const CACHE_TTL_MS = 60 * 1000

export function clearOriginCache() {
  originCache.clear()
}

export async function getAllowedOrigins(tenant) {
  if (!tenant) return []
  const now = Date.now()
  const cached = originCache.get(tenant)
  if (cached && cached.expiresAt > now) {
    return cached.origins
  }

  const backendUrl = process.env.API_URL || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  try {
    const res = await fetch(`${backendUrl}/t/${encodeURIComponent(tenant)}/public-config`, {
      headers: { Accept: 'application/json' },
    })
    if (!res.ok) {
      originCache.set(tenant, { origins: [], expiresAt: now + CACHE_TTL_MS })
      return []
    }
    const data = await res.json()
    const origins = Array.isArray(data?.allowed_embed_origins) ? data.allowed_embed_origins : []
    originCache.set(tenant, { origins, expiresAt: now + CACHE_TTL_MS })
    return origins
  } catch {
    originCache.set(tenant, { origins: [], expiresAt: now + CACHE_TTL_MS })
    return []
  }
}

export async function middleware(request) {
  const pathname = request.nextUrl
    ? request.nextUrl.pathname
    : new URL(request.url, 'http://localhost').pathname

  const response = NextResponse.next()

  // Base security headers applied to all routes
  response.headers.set('X-Content-Type-Options', 'nosniff')
  response.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin')

  const embedMatch = pathname.match(/^\/embed\/([^/]+)/)
  if (embedMatch) {
    const tenant = embedMatch[1]
    const origins = await getAllowedOrigins(tenant)
    if (origins && origins.length > 0) {
      response.headers.set('Content-Security-Policy', `frame-ancestors ${origins.join(' ')}`)
      // Omit X-Frame-Options for allowed embed origins
      response.headers.delete('X-Frame-Options')
    } else {
      // Fail closed to 'none' and DENY if fetch fails or origins list is empty
      response.headers.set('Content-Security-Policy', "frame-ancestors 'none'")
      response.headers.set('X-Frame-Options', 'DENY')
    }
  } else {
    // Standard routes: deny all framing
    response.headers.set('Content-Security-Policy', "frame-ancestors 'none'")
    response.headers.set('X-Frame-Options', 'DENY')
  }

  return response
}

export const config = {
  matcher: [
    /*
     * Match all request paths except static files and images:
     */
    '/((?!_next/static|_next/image|favicon.ico).*)',
  ],
}
