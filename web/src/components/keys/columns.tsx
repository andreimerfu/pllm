"use client"

import { ColumnDef } from "@tanstack/react-table"
import { Icon } from "@iconify/react"
import { icons } from "@/lib/icons"
import { Button } from "../ui/button"
import { Badge } from "../ui/badge"
import { Progress } from "../ui/progress"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "../ui/tooltip"
import { useState } from "react"
import { useToast } from "../ui/use-toast"

export interface ApiKey {
  id: string
  key?: string
  key_prefix?: string
  name: string
  user_id?: string
  team_id?: string
  user?: {
    id: string
    username: string
    email: string
    full_name?: string
  }
  team?: {
    id: string
    name: string
  }
  is_active: boolean
  expires_at?: string
  max_budget?: number
  current_spend: number
  tpm?: number
  rpm?: number
  usage_count: number
  total_tokens: number
  last_used_at?: string
  created_at: string
  revoked_at?: string
}

export const getKeyStatus = (key: ApiKey) => {
  if (key.revoked_at) return 'revoked'
  if (key.expires_at && new Date(key.expires_at) < new Date()) return 'expired'
  if (!key.is_active) return 'inactive'
  return 'active'
}

const getBudgetPercentage = (key: ApiKey) => {
  if (!key.max_budget || key.max_budget === 0) return 0
  return (key.current_spend / key.max_budget) * 100
}

const statusConfig: Record<string, { color: string; label: string }> = {
  active: { color: 'bg-emerald-500', label: 'Active' },
  inactive: { color: 'bg-zinc-400', label: 'Inactive' },
  expired: { color: 'bg-amber-500', label: 'Expired' },
  revoked: { color: 'bg-red-500', label: 'Revoked' },
}

const StatusDot = ({ status }: { status: string }) => {
  const config = statusConfig[status] || statusConfig.inactive
  return (
    <div className="flex items-center gap-2">
      <span className={`inline-block h-2 w-2 rounded-full ${config.color}`} />
      <span className="text-sm capitalize">{config.label}</span>
    </div>
  )
}

const formatRelativeTime = (dateString?: string) => {
  if (!dateString) return 'Never'
  const date = new Date(dateString)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSecs = Math.floor(diffMs / 1000)
  const diffMins = Math.floor(diffSecs / 60)
  const diffHours = Math.floor(diffMins / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffSecs < 60) return 'Just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 30) return `${diffDays}d ago`
  return date.toLocaleDateString()
}

const KeyValueCell = ({ keyValue, keyPrefix }: { keyValue?: string, keyPrefix?: string }) => {
  const { toast } = useToast()
  const [copied, setCopied] = useState(false)

  const displayValue = keyPrefix ? `sk-...${keyPrefix}` : 'sk-...****'

  const copyKey = () => {
    if (keyValue) {
      navigator.clipboard.writeText(keyValue)
      setCopied(true)
      toast({
        title: 'Copied',
        description: 'Key copied to clipboard',
      })
      setTimeout(() => setCopied(false), 2000)
    }
  }

  return (
    <div className="flex items-center gap-1.5 group/key">
      <code className="font-mono text-[11px] text-muted-foreground">
        {displayValue}
      </code>
      <TooltipProvider delayDuration={0}>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="sm"
              disabled={!keyValue}
              onClick={(e) => {
                e.stopPropagation()
                copyKey()
              }}
              className="h-5 w-5 p-0 opacity-0 group-hover/key:opacity-100 transition-opacity"
            >
              <Icon icon={copied ? icons.check : icons.copy} className="h-3 w-3" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            <p className="text-xs">{copied ? 'Copied!' : 'Copy key'}</p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>
  )
}

// Expanded row detail panel
export const ExpandedRowContent = ({ apiKey }: { apiKey: ApiKey }) => {
  const budgetPercentage = getBudgetPercentage(apiKey)

  return (
    <div className="px-6 py-4 bg-muted/30 border-t border-border/50">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-6">
        <div className="space-y-1">
          <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Created</p>
          <p className="text-sm">{new Date(apiKey.created_at).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })}</p>
          <p className="text-xs text-muted-foreground">{new Date(apiKey.created_at).toLocaleTimeString()}</p>
        </div>

        <div className="space-y-1">
          <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Expiration</p>
          <p className="text-sm">
            {apiKey.expires_at
              ? new Date(apiKey.expires_at).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
              : 'Never'}
          </p>
          {apiKey.expires_at && (
            <p className="text-xs text-muted-foreground">{formatRelativeTime(apiKey.expires_at)}</p>
          )}
        </div>

        <div className="space-y-1">
          <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Rate Limits</p>
          <div className="space-y-0.5">
            <p className="text-sm font-mono">
              {apiKey.tpm ? `${apiKey.tpm.toLocaleString()} TPM` : 'Unlimited TPM'}
            </p>
            <p className="text-sm font-mono">
              {apiKey.rpm ? `${apiKey.rpm.toLocaleString()} RPM` : 'Unlimited RPM'}
            </p>
          </div>
        </div>

        <div className="space-y-1">
          <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Budget</p>
          {apiKey.max_budget ? (
            <div className="space-y-1.5">
              <div className="flex justify-between text-sm">
                <span className="font-mono">${apiKey.current_spend.toFixed(2)}</span>
                <span className="text-muted-foreground font-mono">${apiKey.max_budget.toFixed(2)}</span>
              </div>
              <Progress value={budgetPercentage} className="h-1.5" />
            </div>
          ) : (
            <p className="text-sm">No budget limit</p>
          )}
        </div>
      </div>

      {apiKey.user && (
        <div className="mt-4 pt-3 border-t border-border/50">
          <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider mb-1">Owner</p>
          <p className="text-sm">{apiKey.user.full_name || apiKey.user.username} <span className="text-muted-foreground">({apiKey.user.email})</span></p>
        </div>
      )}

      {apiKey.total_tokens > 0 && (
        <div className="mt-3 pt-3 border-t border-border/50">
          <p className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider mb-1">Token Usage</p>
          <p className="text-sm font-mono">{apiKey.total_tokens.toLocaleString()} tokens</p>
        </div>
      )}
    </div>
  )
}

export const createColumns = (
  onToggleStatus: (keyId: string, isActive: boolean) => void,
  onRevokeKey: (keyId: string) => void,
  onDeleteKey: (keyId: string, key: ApiKey) => void
): ColumnDef<ApiKey>[] => [
  {
    accessorKey: "name",
    header: ({ column }) => {
      return (
        <Button
          variant="ghost"
          onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          className="h-8 px-2 lg:px-3"
        >
          Key
          <Icon icon={icons.arrowUpDown} className="ml-2 h-3.5 w-3.5" />
        </Button>
      )
    },
    cell: ({ row }) => {
      const key = row.original
      return (
        <div className="space-y-0.5">
          <div className="font-semibold text-sm">{key.name}</div>
          <KeyValueCell
            keyValue={key.key}
            keyPrefix={key.key_prefix}
          />
        </div>
      )
    },
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => {
      const status = getKeyStatus(row.original)
      return <StatusDot status={status} />
    },
    filterFn: (row, _id, value) => {
      if (!value || value === 'all') return true
      const status = getKeyStatus(row.original)
      return status === value
    },
  },
  {
    accessorKey: "team",
    header: "Team",
    cell: ({ row }) => {
      const key = row.original
      if (key.team) {
        return (
          <Badge variant="outline" className="font-normal text-xs">
            {key.team.name}
          </Badge>
        )
      }
      return <span className="text-xs text-muted-foreground">Personal</span>
    },
    filterFn: (row, _id, value) => {
      if (!value || value === 'all') return true
      const key = row.original
      if (value === 'personal') return !key.team_id
      return key.team_id === value
    },
  },
  {
    accessorKey: "usage_count",
    header: ({ column }) => {
      return (
        <Button
          variant="ghost"
          onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          className="h-8 px-2 lg:px-3"
        >
          Requests
          <Icon icon={icons.arrowUpDown} className="ml-2 h-3.5 w-3.5" />
        </Button>
      )
    },
    cell: ({ row }) => {
      return (
        <span className="font-mono text-sm tabular-nums">
          {row.original.usage_count.toLocaleString()}
        </span>
      )
    },
    sortingFn: (rowA, rowB) => {
      return rowA.original.usage_count - rowB.original.usage_count
    },
  },
  {
    accessorKey: "current_spend",
    header: ({ column }) => {
      return (
        <Button
          variant="ghost"
          onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          className="h-8 px-2 lg:px-3"
        >
          Spend
          <Icon icon={icons.arrowUpDown} className="ml-2 h-3.5 w-3.5" />
        </Button>
      )
    },
    cell: ({ row }) => {
      return (
        <span className="font-mono text-sm tabular-nums">
          ${row.original.current_spend.toFixed(2)}
        </span>
      )
    },
    sortingFn: (rowA, rowB) => {
      return rowA.original.current_spend - rowB.original.current_spend
    },
  },
  {
    accessorKey: "last_used_at",
    header: "Last Used",
    cell: ({ row }) => {
      const lastUsed = row.original.last_used_at
      return (
        <span className="text-sm text-muted-foreground">
          {formatRelativeTime(lastUsed)}
        </span>
      )
    },
  },
  {
    id: "actions",
    cell: ({ row }) => {
      const key = row.original
      const status = getKeyStatus(key)

      return (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              className="h-8 w-8 p-0"
              onClick={(e) => e.stopPropagation()}
            >
              <span className="sr-only">Open menu</span>
              <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>Actions</DropdownMenuLabel>
            {key.key && (
              <DropdownMenuItem
                onClick={(e) => {
                  e.stopPropagation()
                  navigator.clipboard.writeText(key.key!)
                }}
              >
                <Icon icon={icons.copy} className="mr-2 h-4 w-4" />
                Copy Full Key
              </DropdownMenuItem>
            )}
            <DropdownMenuSeparator />
            {status !== 'revoked' && (
              <DropdownMenuItem
                onClick={(e) => {
                  e.stopPropagation()
                  onToggleStatus(key.id, key.is_active)
                }}
              >
                <Icon icon={key.is_active ? icons.power : icons.check} className="mr-2 h-4 w-4" />
                {key.is_active ? 'Disable' : 'Enable'}
              </DropdownMenuItem>
            )}
            {status === 'active' && (
              <DropdownMenuItem
                className="text-destructive"
                onClick={(e) => {
                  e.stopPropagation()
                  onRevokeKey(key.id)
                }}
              >
                <Icon icon={icons.lock} className="mr-2 h-4 w-4" />
                Revoke
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              className="text-destructive"
              onClick={(e) => {
                e.stopPropagation()
                onDeleteKey(key.id, key)
              }}
            >
              <Icon icon={icons.delete} className="mr-2 h-4 w-4" />
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )
    },
  },
]
