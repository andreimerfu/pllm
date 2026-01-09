"use client"

import { ColumnDef } from "@tanstack/react-table"
import { ArrowUpDown } from "lucide-react"
import { Button } from "../ui/button"
import { Badge } from "../ui/badge"
import { Icon } from '@iconify/react'
import { formatDate } from '@/lib/date-utils'
import { User } from '@/types/api'

const getProviderIcon = (providerIcon?: string) => {
  if (!providerIcon) return null

  if (providerIcon === 'key') {
    return <Icon icon="lucide:key" width="18" height="18" className="text-muted-foreground" />
  }

  if (providerIcon === 'user') {
    return <Icon icon="lucide:user" width="18" height="18" className="text-muted-foreground" />
  }

  return <Icon icon={`simple-icons:${providerIcon}`} width="18" height="18" className="text-muted-foreground" />
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
          <ArrowUpDown className="ml-2 h-4 w-4" />
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
      return <div className="flex justify-center">{getProviderIcon(user.provider_icon)}</div>
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
          <ArrowUpDown className="ml-2 h-4 w-4" />
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
