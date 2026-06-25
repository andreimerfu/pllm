import { Button } from '@/components/ui/button'
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import { useUsers } from '@/hooks/useUsers'
import { LoadingState } from '@/components/common/LoadingState'
import { EmptyState } from '@/components/common/EmptyState'
import { DataTable } from '@/components/common/DataTable'
import { createColumns } from '@/components/users/columns'
import { PageShell, StatTile } from '@/components/common/page-shell'

export default function Users() {
  const { users, isLoading, refetch } = useUsers()
  const columns = createColumns()

  if (isLoading) {
    return <LoadingState text="Loading users..." />;
  }

  const admins = users.filter((u) => u.is_admin).length
  const signedIn = users.filter((u) => u.last_login_at || u.last_login).length
  const providers = new Set(
    users.map((u) => u.external_provider).filter(Boolean) as string[],
  ).size

  return (
    <PageShell
      title="Users"
      description="Identities provisioned through Dex OIDC. Roles drive admin-scope checks across the console."
      actions={
        <Button onClick={() => refetch()} variant="outline" disabled={isLoading}>
          <Icon icon={icons.refresh} className={`mr-2 h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      }
    >
      {users.length === 0 ? (
        <EmptyState
          icon={icons.users}
          title="No users found"
          description="No users have been authenticated through Dex yet."
          className="mt-2"
        />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <StatTile
              label="Users"
              value={users.length}
              icon={<Icon icon={icons.user} />}
              iconColor="agent"
              sub="Provisioned via Dex"
            />
            <StatTile
              label="Signed in"
              value={signedIn}
              tone={signedIn > 0 ? "good" : "default"}
              icon={<Icon icon={icons.guardrails} />}
              iconColor="good"
              sub="Logged in at least once"
            />
            <StatTile
              label="Admins"
              value={admins}
              icon={<Icon icon={icons.keys} />}
              iconColor="model"
              sub="Full console access"
            />
            <StatTile
              label="Providers"
              value={providers}
              icon={<Icon icon={icons.globe} />}
              iconColor="mcp"
              sub="Distinct identity sources"
            />
          </div>

          <DataTable columns={columns} data={users} searchPlaceholder="Search users..." />
        </>
      )}
    </PageShell>
  )
}
