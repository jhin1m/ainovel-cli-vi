import { createFileRoute } from '@tanstack/react-router'
import { LibraryPage } from '@/features/books/library-page'

export const Route = createFileRoute('/_app/')({ component: LibraryPage })
