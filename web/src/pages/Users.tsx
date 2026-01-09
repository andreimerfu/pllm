import { Button } from '@/components/ui/button'
import { RefreshCw, Users as UsersIcon } from 'lucide-react'
import { useUsers } from '@/hooks/useUsers'
import { LoadingState } from '@/components/common/LoadingState'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { DataTable } from '@/components/common/DataTable'
import { createColumns } from '@/components/users/columns'

export default function Users() {
  const { users, isLoading, refetch } = useUsers()
  const columns = createColumns()

  if (isLoading) {
    return <LoadingState text="Loading users..." />;
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Users"
        description="View users authenticated through Dex"
        actions={
          <Button
            onClick={() => refetch()}
            variant="outline"
            disabled={isLoading}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        }
      />

      {users.length === 0 ? (
        <EmptyState
          icon={UsersIcon}
          title="No users found"
          description="No users have been authenticated through Dex yet."
        />
      ) : (
        <DataTable columns={columns} data={users} searchPlaceholder="Search users..." />
      )}
    </div>
  )
}
