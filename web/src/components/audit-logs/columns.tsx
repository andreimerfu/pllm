"use client"

import { ColumnDef } from "@tanstack/react-table"
import { ArrowUpDown } from "lucide-react"
import { format } from "date-fns"
import { Button } from "../ui/button"
import { Badge } from "../ui/badge"
import { AuditLog } from '@/types/api'

export const getStatusBadge = (result: string) => {
  switch (result) {
    case 'success':
      return <Badge variant="default" className="bg-green-100 text-green-800 border-green-200">Success</Badge>
    case 'failure':
      return <Badge variant="destructive">Failure</Badge>
    case 'error':
      return <Badge variant="destructive" className="bg-red-100 text-red-800 border-red-200">Error</Badge>
    case 'warning':
      return <Badge variant="secondary" className="bg-yellow-100 text-yellow-800 border-yellow-200">Warning</Badge>
    default:
      return <Badge variant="outline">{result}</Badge>
  }
}

export const getSeverityColor = (eventType: string) => {
  const securityEvents = ['auth', 'login', 'logout', 'password_change', 'security_alert', 'access_denied']
  const highRiskEvents = ['budget_exceeded', 'key_revoke', 'user_delete']

  if (securityEvents.includes(eventType)) return 'text-red-600'
  if (highRiskEvents.includes(eventType)) return 'text-orange-600'
  return 'text-gray-600'
}

export const createAuditColumns = (onRowClick?: (log: AuditLog) => void): ColumnDef<AuditLog>[] => [
  {
    accessorKey: "timestamp",
    header: ({ column }) => (
      <Button
        variant="ghost"
        onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
        className="p-0 h-auto"
      >
        Time
        <ArrowUpDown className="ml-2 h-4 w-4" />
      </Button>
    ),
    cell: ({ row }) => {
      const timestamp = new Date(row.getValue("timestamp"))
      return (
        <div className="text-sm">
          <div>{format(timestamp, "MMM dd, yyyy")}</div>
          <div className="text-muted-foreground text-xs">{format(timestamp, "HH:mm:ss")}</div>
        </div>
      )
    },
  },
  {
    accessorKey: "user",
    header: "User",
    cell: ({ row }) => {
      const auditLog = row.original
      return (
        <div className="text-sm">
          {auditLog.user ? (
            <>
              <div>{auditLog.user.name || auditLog.user.email || 'Unknown User'}</div>
              {auditLog.user.email && <div className="text-muted-foreground text-xs">{auditLog.user.email}</div>}
            </>
          ) : (
            <span className="text-muted-foreground">System</span>
          )}
        </div>
      )
    },
  },
  {
    accessorKey: "event_action",
    header: "Action",
    cell: ({ row }) => {
      const auditLog = row.original
      return (
        <div className="text-sm">
          <div className={`font-medium ${getSeverityColor(auditLog.event_type)}`}>
            {auditLog.event_action}
          </div>
          <div className="text-muted-foreground text-xs capitalize">
            {auditLog.event_type.replace(/_/g, ' ')}
          </div>
        </div>
      )
    },
  },
  {
    accessorKey: "resource_type",
    header: "Resource",
    cell: ({ row }) => {
      const auditLog = row.original
      return auditLog.resource_type ? (
        <div className="text-sm">
          <div className="font-medium capitalize">{auditLog.resource_type}</div>
          {auditLog.resource_id && (
            <div className="text-muted-foreground text-xs font-mono">
              {auditLog.resource_id.slice(0, 8)}...
            </div>
          )}
        </div>
      ) : (
        <span className="text-muted-foreground">-</span>
      )
    },
  },
  {
    accessorKey: "event_result",
    header: "Result",
    cell: ({ row }) => getStatusBadge(row.getValue("event_result")),
  },
  {
    accessorKey: "ip_address",
    header: "IP Address",
    cell: ({ row }) => (
      <div className="text-sm font-mono">{row.getValue("ip_address") || "-"}</div>
    ),
  },
  {
    accessorKey: "method",
    header: "Method",
    cell: ({ row }) => {
      const method = row.getValue("method") as string
      if (!method) return <span className="text-muted-foreground">-</span>

      const methodColors = {
        GET: "bg-blue-100 text-blue-800 border-blue-200",
        POST: "bg-green-100 text-green-800 border-green-200",
        PUT: "bg-yellow-100 text-yellow-800 border-yellow-200",
        DELETE: "bg-red-100 text-red-800 border-red-200",
      }

      return (
        <Badge variant="outline" className={methodColors[method as keyof typeof methodColors] || ""}>
          {method}
        </Badge>
      )
    },
  },
  {
    id: "actions",
    cell: ({ row }) => {
      return (
        <Button
          variant="ghost"
          size="sm"
          className="hover:bg-muted transition-colors"
          onClick={() => onRowClick?.(row.original)}
        >
          View
        </Button>
      )
    },
  },
]
