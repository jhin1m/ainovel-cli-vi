import type { QueryClient } from '@tanstack/react-query'
import { redirect } from '@tanstack/react-router'
import { ApiError } from '@/api/client'
import { meQuery } from '@/api/queries'

/** Route cần đăng nhập: chưa đăng nhập (401) thì chuyển sang /login. */
export async function requireUser(qc: QueryClient): Promise<void> {
  try {
    await qc.ensureQueryData(meQuery)
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      qc.removeQueries({ queryKey: meQuery.queryKey })
      throw redirect({ to: '/login' })
    }
    throw e
  }
}

/** Trang /login, /signup: đã đăng nhập thì về thư viện. */
export async function redirectIfSignedIn(qc: QueryClient): Promise<void> {
  try {
    await qc.ensureQueryData(meQuery)
  } catch {
    return // chưa đăng nhập hoặc lỗi mạng: cứ cho hiện form
  }
  throw redirect({ to: '/' })
}
