"use client"

import { ColumnDef } from "@tanstack/react-table"
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import { Button } from "../ui/button"
import { Badge } from "../ui/badge"
import { formatDate } from '@/lib/date-utils'
import { User } from '@/types/api'

const PROVIDER_ICON_MAP: Record<string, { icon: string; label: string }> = {
  github: { icon: 'simple-icons:github', label: 'GitHub' },
  gitlab: { icon: 'simple-icons:gitlab', label: 'GitLab' },
  bitbucket: { icon: 'simple-icons:bitbucket', label: 'Bitbucket' },
  microsoft: { icon: 'simple-icons:microsoft', label: 'Microsoft' },
  google: { icon: 'simple-icons:google', label: 'Google' },
  okta: { icon: 'simple-icons:okta', label: 'Okta' },
  auth0: { icon: 'simple-icons:auth0', label: 'Auth0' },
  keycloak: { icon: 'simple-icons:keycloak', label: 'Keycloak' },
  ldap: { icon: 'solar:server-2-linear', label: 'LDAP' },
  saml: { icon: 'solar:shield-keyhole-linear', label: 'SAML' },
}

const normalizeProvider = (raw?: string): string | null => {
  if (!raw) return null
  const p = raw.trim().toLowerCase()
  if (!p) return null
  if (PROVIDER_ICON_MAP[p]) return p
  if (p.includes('github')) return 'github'
  if (p.includes('gitlab')) return 'gitlab'
  if (p.includes('bitbucket')) return 'bitbucket'
  if (p.includes('microsoft') || p.includes('azure') || p.includes('entra') || p === 'ms' || p === 'msft' || p === 'aad') return 'microsoft'
  if (p.includes('google') || p.includes('workspace')) return 'google'
  if (p.includes('okta')) return 'okta'
  if (p.includes('auth0')) return 'auth0'
  if (p.includes('keycloak')) return 'keycloak'
  if (p.includes('ldap')) return 'ldap'
  if (p.includes('saml')) return 'saml'
  return null
}

const getProviderIcon = (user: User) => {
  const normalized = normalizeProvider(user.external_provider) ?? normalizeProvider(user.provider_icon)

  if (normalized && PROVIDER_ICON_MAP[normalized]) {
    const { icon, label } = PROVIDER_ICON_MAP[normalized]
    return (
      <Icon
        icon={icon}
        width="18"
        height="18"
        className="text-muted-foreground"
        aria-label={label}
      />
    )
  }

  const rawIcon = user.provider_icon
  if (rawIcon === 'master_key') {
    return (
      <Icon
        icon={icons.keys}
        width="18"
        height="18"
        className="text-muted-foreground"
        aria-label="Master key"
      />
    )
  }
  if (rawIcon === 'key') {
    return (
      <Icon
        icon={icons.keys}
        width="18"
        height="18"
        className="text-muted-foreground"
        aria-label="Local account"
      />
    )
  }

  return (
    <Icon
      icon={icons.user}
      width="18"
      height="18"
      className="text-muted-foreground"
      aria-label={user.external_provider || 'Unknown provider'}
    />
  )
}

const getUserDisplayName = (user: User) => {
  if (user.first_name && user.last_name) {
    return `${user.first_name} ${user.last_name}`
  }
  return user.name || user.email?.split('@')[0] || 'Unknown'
}

export const createColumns = (): ColumnDef<User>[] => [
  {
    accessorKey: "name",
    header: ({ column }) => {
      return (
        <Button
          variant="ghost"
          onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
        >
          Name
          <Icon icon={icons.arrowUpDown} className="ml-2 h-4 w-4" />
        </Button>
      )
    },
    cell: ({ row }) => {
      const user = row.original
      return <div className="font-medium">{getUserDisplayName(user)}</div>
    },
  },
  {
    accessorKey: "provider_icon",
    header: "Provider",
    cell: ({ row }) => {
      const user = row.original
      return (
        <div
          className="flex justify-center"
          title={user.external_provider || 'Unknown provider'}
        >
          {getProviderIcon(user)}
        </div>
      )
    },
  },
  {
    accessorKey: "email",
    header: "Email",
    cell: ({ row }) => {
      return <div className="text-muted-foreground">{row.getValue("email") || 'No email'}</div>
    },
  },
  {
    accessorKey: "is_admin",
    header: "Role",
    cell: ({ row }) => {
      const isAdmin = row.getValue("is_admin") as boolean
      return (
        <Badge variant={isAdmin ? 'default' : 'secondary'} className="font-medium">
          {isAdmin ? 'Admin' : 'User'}
        </Badge>
      )
    },
  },
  {
    accessorKey: "last_login_at",
    header: ({ column }) => {
      return (
        <Button
          variant="ghost"
          onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
        >
          Last Login
          <Icon icon={icons.arrowUpDown} className="ml-2 h-4 w-4" />
        </Button>
      )
    },
    cell: ({ row }) => {
      const lastLogin = row.getValue("last_login_at") as string | undefined
      return (
        <div className="text-muted-foreground text-xs">
          {lastLogin ? formatDate(lastLogin) : 'Never'}
        </div>
      )
    },
  },
  {
    id: "actions",
    cell: () => {
      return (
        <Button variant="ghost" size="sm" className="hover:bg-muted transition-colors">
          View
        </Button>
      )
    },
  },
]
