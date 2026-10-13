import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'
import type {
  APIKey,
  Book,
  ChangePasswordRequest,
  ChapterInfo,
  ChapterText,
  CommandResult,
  CreateAPIKeyRequest,
  CreateBookRequest,
  LoginRequest,
  Outline,
  ProviderInfo,
  SignupRequest,
  UpdateBookRequest,
  User,
} from './types.gen'

export const qk = {
  me: ['me'] as const,
  keys: ['keys'] as const,
  providers: ['providers'] as const,
  books: ['books'] as const,
  book: (id: string) => ['book', id] as const,
  chapters: (id: string) => ['book', id, 'chapters'] as const,
  chapter: (id: string, n: number) => ['book', id, 'chapter', n] as const,
  outline: (id: string) => ['book', id, 'outline'] as const,
}

const bookPath = (id: string) => `/api/books/${encodeURIComponent(id)}`

// ---------- Queries ----------

export const meQuery = queryOptions({
  queryKey: qk.me,
  queryFn: ({ signal }) => api.get<User>('/api/me', { signal }),
  staleTime: 5 * 60_000,
  retry: false,
})

export const useMe = () => useQuery(meQuery)

export const keysQuery = queryOptions({
  queryKey: qk.keys,
  queryFn: async ({ signal }) => (await api.get<APIKey[] | null>('/api/keys', { signal })) ?? [],
})
export const useKeys = () => useQuery(keysQuery)

export const providersQuery = queryOptions({
  queryKey: qk.providers,
  queryFn: async ({ signal }) =>
    (await api.get<ProviderInfo[] | null>('/api/providers', { signal })) ?? [],
  staleTime: 10 * 60_000,
})
export const useProviders = () => useQuery(providersQuery)

export const useBooks = () =>
  useQuery({
    queryKey: qk.books,
    queryFn: async ({ signal }) => (await api.get<Book[] | null>('/api/books', { signal })) ?? [],
    refetchInterval: 15_000,
  })

export const bookQuery = (id: string) =>
  queryOptions({
    queryKey: qk.book(id),
    queryFn: ({ signal }) => api.get<Book>(bookPath(id), { signal }),
  })
export const useBook = (id: string) => useQuery(bookQuery(id))

export const useChapters = (id: string) =>
  useQuery({
    queryKey: qk.chapters(id),
    queryFn: async ({ signal }) =>
      (await api.get<ChapterInfo[] | null>(`${bookPath(id)}/chapters`, { signal })) ?? [],
  })

export const useChapter = (id: string, n: number) =>
  useQuery({
    queryKey: qk.chapter(id, n),
    queryFn: ({ signal }) => api.get<ChapterText>(`${bookPath(id)}/chapters/${n}`, { signal }),
    enabled: Number.isInteger(n) && n > 0,
    staleTime: 60_000,
  })

/** `enabled` để chỉ gọi khi mở tab Dàn ý. */
export const useOutline = (id: string, enabled = true) =>
  useQuery({
    queryKey: qk.outline(id),
    queryFn: ({ signal }) => api.get<Outline>(`${bookPath(id)}/outline`, { signal }),
    enabled,
  })

// ---------- Mutations ----------

export function useLogin() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: LoginRequest) => api.post<User>('/api/auth/login', req),
    onSuccess: (u) => {
      qc.clear()
      qc.setQueryData(qk.me, u)
    },
  })
}

export function useSignup() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: SignupRequest) => api.post<User>('/api/auth/signup', req),
    onSuccess: (u) => {
      qc.clear()
      qc.setQueryData(qk.me, u)
    },
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post<void>('/api/auth/logout'),
    onSettled: () => qc.clear(),
  })
}

export const useChangePassword = () =>
  useMutation({
    mutationFn: (req: ChangePasswordRequest) => api.post<void>('/api/me/password', req),
  })

export function useCreateKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateAPIKeyRequest) => api.post<APIKey>('/api/keys', req),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.keys }),
  })
}

export function useDeleteKey() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.del<void>(`/api/keys/${encodeURIComponent(id)}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.keys }),
  })
}

export function useCreateBook() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateBookRequest) => api.post<Book>('/api/books', req),
    onSuccess: (b) => {
      qc.setQueryData(qk.book(b.id), b)
      void qc.invalidateQueries({ queryKey: qk.books })
    },
  })
}

export function usePatchBook(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (req: UpdateBookRequest) => api.patch<Book>(bookPath(id), req),
    onSuccess: (b) => {
      qc.setQueryData(qk.book(id), b)
      void qc.invalidateQueries({ queryKey: qk.books })
    },
  })
}

export type BookCommand = 'pause' | 'resume' | 'next'

/** pause / resume / next: server trả CommandResult kèm Book mới. */
export function useBookCommand(id: string, cmd: BookCommand) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api.post<CommandResult>(`${bookPath(id)}/${cmd}`),
    onSuccess: (r) => {
      qc.setQueryData(qk.book(id), r.book)
      void qc.invalidateQueries({ queryKey: qk.books })
    },
  })
}
export const usePauseBook = (id: string) => useBookCommand(id, 'pause')
export const useResumeBook = (id: string) => useBookCommand(id, 'resume')
export const useNextChapter = (id: string) => useBookCommand(id, 'next')

export function useSteer(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (text: string) => api.post<CommandResult>(`${bookPath(id)}/steer`, { text }),
    onSuccess: (r) => qc.setQueryData(qk.book(id), r.book),
  })
}
