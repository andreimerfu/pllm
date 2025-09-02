import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Icon } from '@iconify/react'
import { useState, useEffect } from 'react'
import { User } from '@/types/api'
import api from '@/lib/api'
import { useToast } from '@/hooks/use-toast'
import { Loader2, RefreshCw } from 'lucide-react'

export default function Users() {
  const [searchTerm, setSearchTerm] = useState('')
  const [users, setUsers] = useState<User[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const { toast } = useToast()

  const fetchUsers = async () => {
    try {
      setIsLoading(true)
      const response = await api.users.list()
      setUsers(response as any)
    } catch (error) {
      console.error('Failed to fetch users:', error)
      toast({
        title: 'Error',
        description: 'Failed to load users',
        variant: 'destructive',
      })
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    fetchUsers()
  }, [])

  const filteredUsers = users.filter(user => {
    const searchLower = searchTerm.toLowerCase();
    const fullName = user.first_name && user.last_name ? `${user.first_name} ${user.last_name}` : '';
    const displayName = fullName || user.name || user.email?.split('@')[0] || '';
    
    return (
      displayName.toLowerCase().includes(searchLower) ||
      (user.first_name?.toLowerCase().includes(searchLower) || false) ||
      (user.last_name?.toLowerCase().includes(searchLower) || false) ||
      (user.email?.toLowerCase().includes(searchLower) || false)
    );
  })

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold bg-gradient-to-r from-foreground to-foreground/70 bg-clip-text text-transparent">
            Users
          </h1>
          <p className="text-sm lg:text-base text-muted-foreground mt-1">View users authenticated through Dex</p>
        </div>
        <Button 
          onClick={fetchUsers}
          variant="outline"
          className="w-full sm:w-auto shadow-lg hover:shadow-xl transition-all duration-200"
          disabled={isLoading}
        >
          <RefreshCw className={`mr-2 h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      <Card className="transition-theme">
        <CardHeader>
          <CardTitle className="text-lg lg:text-xl">User Management</CardTitle>
          <CardDescription>View and manage all user accounts</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-4">
            <div className="relative">
              <Icon icon="lucide:search" width="16" height="16" className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="Search users by name or email..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="pl-10"
                disabled={isLoading}
              />
            </div>
          </div>

          {isLoading && (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin" />
            </div>
          )}

          {!isLoading && users.length === 0 && (
            <div className="text-center py-8">
              <p className="text-muted-foreground">No users found</p>
            </div>
          )}

          {!isLoading && filteredUsers.length === 0 && users.length > 0 && (
            <div className="text-center py-8">
              <p className="text-muted-foreground">No users match your search</p>
            </div>
          )}

          {/* Mobile Card View */}
          {!isLoading && filteredUsers.length > 0 && (
            <div className="block sm:hidden space-y-3">
              {filteredUsers.map(user => (
                <Card key={user.id} className="p-4 transition-theme">
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      {user.provider_icon && (
                        <Icon 
                          icon={`simple-icons:${user.provider_icon}`} 
                          width="16" 
                          height="16" 
                          className="text-muted-foreground" 
                        />
                      )}
                      {user.provider_icon === 'key' && (
                        <Icon 
                          icon="lucide:key" 
                          width="16" 
                          height="16" 
                          className="text-muted-foreground" 
                        />
                      )}
                      {user.provider_icon === 'user' && (
                        <Icon 
                          icon="lucide:user" 
                          width="16" 
                          height="16" 
                          className="text-muted-foreground" 
                        />
                      )}
                      <h3 className="font-semibold">
                        {user.first_name && user.last_name 
                          ? `${user.first_name} ${user.last_name}` 
                          : user.name || user.email?.split('@')[0] || 'Unknown'
                        }
                      </h3>
                    </div>
                    <Badge variant={user.is_admin ? 'default' : 'secondary'} className="font-medium">
                      {user.is_admin ? 'Admin' : 'User'}
                    </Badge>
                  </div>
                  <p className="text-sm text-muted-foreground mb-1">{user.email || 'No email'}</p>
                  <p className="text-xs text-muted-foreground mb-2">
                    Last Login: {user.last_login_at ? new Date(user.last_login_at).toLocaleDateString() : 'Never'}
                  </p>
                  <div className="flex items-center justify-between">
                    <Badge 
                      variant="outline"
                      className="border-green-200 text-green-700 dark:border-green-800 dark:text-green-400"
                    >
                      Active
                    </Badge>
                    <div className="flex space-x-1">
                      <Button variant="ghost" size="sm" className="hover:bg-muted transition-colors">
                        View
                      </Button>
                    </div>
                  </div>
                </Card>
              ))}
            </div>
          )}

          {/* Desktop Table View */}
          {!isLoading && filteredUsers.length > 0 && (
            <div className="hidden sm:block overflow-x-auto">
              <div className="min-w-[800px]">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border">
                      <th className="text-left p-3 font-semibold min-w-[180px]">Name</th>
                      <th className="text-left p-3 font-semibold min-w-[80px]">Provider</th>
                      <th className="text-left p-3 font-semibold min-w-[250px]">Email</th>
                      <th className="text-left p-3 font-semibold min-w-[80px]">Role</th>
                      <th className="text-left p-3 font-semibold min-w-[120px]">Last Login</th>
                      <th className="text-left p-3 font-semibold min-w-[100px]">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredUsers.map(user => (
                      <tr key={user.id} className="border-b border-border/50 hover:bg-muted/30 transition-colors duration-200">
                        <td className="p-3 font-medium">
                          {user.first_name && user.last_name 
                            ? `${user.first_name} ${user.last_name}` 
                            : user.name || user.email?.split('@')[0] || 'Unknown'
                          }
                        </td>
                        <td className="p-3">
                          {user.provider_icon && (
                            <Icon 
                              icon={`simple-icons:${user.provider_icon}`} 
                              width="18" 
                              height="18" 
                              className="text-muted-foreground" 
                            />
                          )}
                          {user.provider_icon === 'key' && (
                            <Icon 
                              icon="lucide:key" 
                              width="18" 
                              height="18" 
                              className="text-muted-foreground" 
                            />
                          )}
                          {user.provider_icon === 'user' && (
                            <Icon 
                              icon="lucide:user" 
                              width="18" 
                              height="18" 
                              className="text-muted-foreground" 
                            />
                          )}
                        </td>
                        <td className="p-3 text-muted-foreground">{user.email || 'No email'}</td>
                        <td className="p-3">
                          <Badge variant={user.is_admin ? 'default' : 'secondary'} className="font-medium">
                            {user.is_admin ? 'Admin' : 'User'}
                          </Badge>
                        </td>
                        <td className="p-3 text-muted-foreground text-xs">
                          {user.last_login_at ? new Date(user.last_login_at).toLocaleDateString() : 'Never'}
                        </td>
                        <td className="p-3">
                          <Button variant="ghost" size="sm" className="hover:bg-muted transition-colors">
                            View
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}