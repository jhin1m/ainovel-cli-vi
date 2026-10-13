import { createFileRoute } from '@tanstack/react-router'
import { AccountPage } from '@/features/keys/account-page'

export const Route = createFileRoute('/_app/settings/account')({ component: AccountPage })
