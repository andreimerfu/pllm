"use client"

import { ColumnDef } from "@tanstack/react-table"
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import { Button } from "../ui/button"
import { StatusChip } from "@/components/common/status-chip"
import { formatRelativeTime } from '@/lib/date-utils'
import { User } from '@/types/api'

const getUserDisplayName = (user: User) => {
  if (user.first_name && user.last_name) {
    return `${user.first_name} ${user.last_name}`
  }
  return user.name || user.email?.split('@')[0] || 'Unknown'
}

// Deterministic avatar tint + initials — display logic only.
const AVATAR_TINTS = [
  "bg-info/12 text-info",
  "bg-good/12 text-good",
  "bg-warn/12 text-warn",
  "bg-bad/12 text-bad",
  "bg-kind-agent/12 text-kind-agent",
  "bg-kind-prompt/12 text-kind-prompt",
  "bg-kind-skill/12 text-kind-skill",
  "bg-kind-model/12 text-kind-model",
]

const avatarTint = (seed: string) => {
  let hash = 0
  for (let i = 0; i < seed.length; i++) {
    hash = (hash * 31 + seed.charCodeAt(i)) | 0
  }
  return AVATAR_TINTS[Math.abs(hash) % AVATAR_TINTS.length]
}

const getInitials = (name: string) => {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

// "USER" header label — mono uppercase eyebrow, dim. Returns a node (not a
// component) so the file keeps a single component-shaped export.
const colHeader = (label: string) => (
  <span className="font-mono text-[10.5px] font-semibold uppercase tracking-[0.08em] text-text-dim">
    {label}
  </span>
)

export const createColumns = (): ColumnDef<User>[] => [
  {
    accessorKey: "name",
    header: () => colHeader("User"),
    cell: ({ row }) => {
      const user = row.original
      const displayName = getUserDisplayName(user)
      return (
        <div className="flex items-center gap-3">
          <span
            className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-[12px] font-semibold ${avatarTint(
              displayName || user.id,
            )}`}
          >
            {getInitials(displayName)}
          </span>
          <div className="min-w-0">
            <div className="truncate font-semibold text-foreground">{displayName}</div>
            <div className="truncate text-[12px] text-muted-foreground">
              {user.email || 'No email'}
            </div>
          </div>
        </div>
      )
    },
  },
  {
    accessorKey: "is_admin",
    header: () => colHeader("Role"),
    cell: ({ row }) => {
      const isAdmin = row.getValue("is_admin") as boolean
      return (
        <StatusChip
          tone={isAdmin ? 'info' : 'neutral'}
          dot={false}
          label={isAdmin ? 'Admin' : 'Member'}
        />
      )
    },
  },
  {
    accessorKey: "groups",
    header: () => colHeader("Teams"),
    cell: ({ row }) => {
      const groups = row.original.groups
      const hasGroups = Array.isArray(groups) && groups.length > 0
      return (
        <span className={hasGroups ? "text-[13px] text-foreground" : "text-text-dim"}>
          {hasGroups ? groups.join(', ') : '—'}
        </span>
      )
    },
  },
  {
    accessorKey: "last_login_at",
    header: () => colHeader("Last active"),
    cell: ({ row }) => {
      const user = row.original
      const lastActive = user.last_login_at || user.last_login
      return (
        <span className="font-mono text-[12px] text-muted-foreground">
          {lastActive ? formatRelativeTime(lastActive) : 'Never'}
        </span>
      )
    },
  },
  {
    id: "status",
    header: () => colHeader("Status"),
    cell: ({ row }) => {
      const user = row.original
      const lastActive = user.last_login_at || user.last_login
      if (lastActive) {
        return <StatusChip tone="good" label="Active" />
      }
      return <StatusChip tone="warn" label="Invited" />
    },
  },
  {
    id: "actions",
    header: () => null,
    cell: () => {
      return (
        <div className="flex justify-end">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8 text-text-dim hover:bg-surface-2 hover:text-foreground"
            aria-label="User actions"
          >
            <Icon icon={icons.moreVertical} className="h-4 w-4" />
          </Button>
        </div>
      )
    },
  },
]
