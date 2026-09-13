import { createFileRoute, redirect } from '@tanstack/react-router'

import { WithdrawalSettings } from '@/features/withdrawal-settings'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/withdrawal-settings/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || !auth.accessToken) {
      throw redirect({ to: '/sign-in' })
    }
    if (auth.user.role !== ROLE.SUPER_ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: WithdrawalSettings,
})
