"use client"

import { ColumnDef } from "@tanstack/react-table"
import { ArrowUpDown, Copy, Eye, EyeOff, MoreHorizontal, Power, Trash2, RotateCw } from "lucide-react"
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

const getKeyStatus = (key: ApiKey) => {
  if (key.revoked_at) return 'revoked'
  if (key.expires_at && new Date(key.expires_at) < new Date()) return 'expired'
  if (!key.is_active) return 'inactive'
  return 'active'
}

const getBudgetPercentage = (key: ApiKey) => {
  if (!key.max_budget || key.max_budget === 0) return 0
  return (key.current_spend / key.max_budget) * 100
}

const StatusBadge = ({ status }: { status: string }) => {
  const variant = status === 'active' ? 'default' :
                 status === 'inactive' ? 'secondary' :
                 status === 'expired' ? 'outline' :
                 status === 'revoked' ? 'destructive' : 'outline'
  
  return <Badge variant={variant}>{status}</Badge>
}

const KeyCell = ({ keyValue, keyPrefix }: { keyValue?: string, keyPrefix?: string }) => {
  const [showKey, setShowKey] = useState(false)
  const { toast } = useToast()

  const copyKey = () => {
    if (keyValue) {
      navigator.clipboard.writeText(keyValue)
      toast({
        title: 'Copied',
        description: 'Key copied to clipboard',
      })
    }
  }

  return (
    <div className="flex items-center gap-2 font-mono text-sm">
      <code className="bg-muted px-2 py-1 rounded">
        {showKey && keyValue ? keyValue : `****${keyPrefix || 'xxxx'}`}
      </code>
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setShowKey(!showKey)}
        className="h-6 w-6 p-0"
      >
        {showKey ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
      </Button>
      <Button
        variant="ghost"
        size="sm"
        disabled={!keyValue}
        onClick={copyKey}
        className="h-6 w-6 p-0"
      >
        <Copy className="h-3 w-3" />
      </Button>
    </div>
  )
}

const UsageCell = ({ apiKey }: { apiKey: ApiKey }) => {
  const budgetPercentage = getBudgetPercentage(apiKey)
  
  return (
    <div className="space-y-1">
      <div className="text-sm">
        {apiKey.usage_count.toLocaleString()} req
      </div>
      {apiKey.max_budget && (
        <div className="space-y-1">
          <div className="flex justify-between text-xs text-muted-foreground">
            <span>${apiKey.current_spend.toFixed(2)}</span>
            <span>${apiKey.max_budget.toFixed(2)}</span>
          </div>
          <Progress 
            value={budgetPercentage} 
            className="h-1"
          />
        </div>
      )}
    </div>
  )
}

const OwnerCell = ({ apiKey }: { apiKey: ApiKey }) => {
  if (apiKey.team) {
    return (
      <div>
        <div className="font-medium">{apiKey.team.name}</div>
        <div className="text-sm text-muted-foreground">Team</div>
      </div>
    )
  }
  
  if (apiKey.user) {
    return (
      <div>
        <div className="font-medium">{apiKey.user.full_name || apiKey.user.username}</div>
        <div className="text-sm text-muted-foreground">{apiKey.user.email}</div>
      </div>
    )
  }
  
  return (
    <div>
      <div className="font-medium">System</div>
      <div className="text-sm text-muted-foreground">System Key</div>
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
          Name
          <ArrowUpDown className="ml-2 h-4 w-4" />
        </Button>
      )
    },
    cell: ({ row }) => {
      const key = row.original
      return (
        <div>
          <div className="font-medium">{key.name}</div>
          <KeyCell 
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
      return <StatusBadge status={status} />
    },
    filterFn: (row, _id, value) => {
      const status = getKeyStatus(row.original)
      return value.includes(status)
    },
  },
  {
    accessorKey: "owner",
    header: "Owner",
    cell: ({ row }) => <OwnerCell apiKey={row.original} />,
  },
  {
    accessorKey: "usage",
    header: ({ column }) => {
      return (
        <Button
          variant="ghost"
          onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          className="h-8 px-2 lg:px-3"
        >
          Usage
          <ArrowUpDown className="ml-2 h-4 w-4" />
        </Button>
      )
    },
    cell: ({ row }) => <UsageCell apiKey={row.original} />,
    sortingFn: (rowA, rowB) => {
      return rowA.original.usage_count - rowB.original.usage_count
    },
  },
  {
    accessorKey: "created_at",
    header: ({ column }) => {
      return (
        <Button
          variant="ghost"
          onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
          className="h-8 px-2 lg:px-3"
        >
          Created
          <ArrowUpDown className="ml-2 h-4 w-4" />
        </Button>
      )
    },
    cell: ({ row }) => {
      const date = new Date(row.getValue("created_at"))
      return (
        <div>
          <div>{date.toLocaleDateString()}</div>
          <div className="text-sm text-muted-foreground">
            {date.toLocaleTimeString()}
          </div>
        </div>
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
            <Button variant="ghost" className="h-8 w-8 p-0">
              <span className="sr-only">Open menu</span>
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>Actions</DropdownMenuLabel>
            {key.key && (
              <DropdownMenuItem
                onClick={() => {
                  navigator.clipboard.writeText(key.key!)
                }}
              >
                <Copy className="mr-2 h-4 w-4" />
                Copy key
              </DropdownMenuItem>
            )}
            <DropdownMenuSeparator />
            {status !== 'revoked' && (
              <DropdownMenuItem
                onClick={() => onToggleStatus(key.id, key.is_active)}
              >
                <Power className="mr-2 h-4 w-4" />
                {key.is_active ? 'Disable' : 'Enable'}
              </DropdownMenuItem>
            )}
            {status === 'active' && (
              <DropdownMenuItem 
                className="text-destructive"
                onClick={() => onRevokeKey(key.id)}
              >
                <RotateCw className="mr-2 h-4 w-4" />
                Revoke
              </DropdownMenuItem>
            )}
            <DropdownMenuItem 
              className="text-destructive"
              onClick={() => onDeleteKey(key.id, key)}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )
    },
  },
]