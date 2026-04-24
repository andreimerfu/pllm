import { Button } from '@/components/ui/button'
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import { useUsers } from '@/hooks/useUsers'
import { LoadingState } from '@/components/common/LoadingState'
import { EmptyState } from '@/components/common/EmptyState'
import { DataTable } from '@/components/common/DataTable'
import { createColumns } from '@/components/users/columns'
import { PageShell } from '@/components/common/page-shell'
import { PageActions } from '@/components/common/page-chrome'

export default function Users() {
  const { users, isLoading, refetch } = useUsers()
  const columns = createColumns()

  if (isLoading) {
    return <LoadingState text="Loading users..." />;
  }

  return (
    <PageShell
      eyebrow="Access"
      title="Authenticated"
      accent="users"
      description="Identities provisioned through Dex OIDC. Roles drive admin-scope checks across the console."
    >
      <PageActions>
        <Button
          onClick={() => refetch()}
          variant="outline"
          disabled={isLoading}
        >
          <Icon icon={icons.refresh} className={`mr-2 h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </PageActions>

      {users.length === 0 ? (
        <EmptyState
          icon={icons.users}
          title="No users found"
          description="No users have been authenticated through Dex yet."
          className="mt-2"
        />
      ) : (
        <DataTable columns={columns} data={users} searchPlaceholder="Search users..." />
      )}
    </PageShell>
  )
}
