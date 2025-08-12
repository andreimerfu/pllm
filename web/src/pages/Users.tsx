import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Icon } from '@iconify/react'
import { useState } from 'react'

export default function Users() {
  const [searchTerm, setSearchTerm] = useState('')

  // Mock data for now
  const users = [
    { id: 1, name: 'Admin User', email: 'admin@example.com', role: 'Admin', status: 'Active' },
    { id: 2, name: 'John Doe', email: 'john@example.com', role: 'User', status: 'Active' },
    { id: 3, name: 'Jane Smith', email: 'jane@example.com', role: 'User', status: 'Inactive' },
  ]

  const filteredUsers = users.filter(user => 
    user.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    user.email.toLowerCase().includes(searchTerm.toLowerCase())
  )

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold bg-gradient-to-r from-foreground to-foreground/70 bg-clip-text text-transparent">
            Users
          </h1>
          <p className="text-sm lg:text-base text-muted-foreground mt-1">Manage user accounts and permissions</p>
        </div>
        <Button className="w-full sm:w-auto shadow-lg hover:shadow-xl transition-all duration-200">
          <Icon icon="lucide:user-plus" width="16" height="16" className="mr-2" />
          Add User
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
                placeholder="Search users..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="pl-10"
              />
            </div>
          </div>

          {/* Mobile Card View */}
          <div className="block sm:hidden space-y-3">
            {filteredUsers.map(user => (
              <Card key={user.id} className="p-4 transition-theme">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="font-semibold">{user.name}</h3>
                  <Badge variant={user.role === 'Admin' ? 'default' : 'secondary'} className="font-medium">
                    {user.role}
                  </Badge>
                </div>
                <p className="text-sm text-muted-foreground mb-2">{user.email}</p>
                <div className="flex items-center justify-between">
                  <Badge 
                    variant={user.status === 'Active' ? 'outline' : 'secondary'}
                    className={user.status === 'Active' ? 'border-green-200 text-green-700 dark:border-green-800 dark:text-green-400' : ''}
                  >
                    {user.status}
                  </Badge>
                  <Button variant="ghost" size="sm" className="hover:bg-muted transition-colors">
                    Edit
                  </Button>
                </div>
              </Card>
            ))}
          </div>

          {/* Desktop Table View */}
          <div className="hidden sm:block overflow-x-auto">
            <div className="min-w-[600px]">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border">
                    <th className="text-left p-3 font-semibold min-w-[120px]">Name</th>
                    <th className="text-left p-3 font-semibold min-w-[200px]">Email</th>
                    <th className="text-left p-3 font-semibold min-w-[80px]">Role</th>
                    <th className="text-left p-3 font-semibold min-w-[80px]">Status</th>
                    <th className="text-left p-3 font-semibold min-w-[100px]">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredUsers.map(user => (
                    <tr key={user.id} className="border-b border-border/50 hover:bg-muted/30 transition-colors duration-200">
                      <td className="p-3 font-medium">{user.name}</td>
                      <td className="p-3 text-muted-foreground">{user.email}</td>
                      <td className="p-3">
                        <Badge variant={user.role === 'Admin' ? 'default' : 'secondary'} className="font-medium">
                          {user.role}
                        </Badge>
                      </td>
                      <td className="p-3">
                        <Badge 
                          variant={user.status === 'Active' ? 'outline' : 'secondary'}
                          className={user.status === 'Active' ? 'border-green-200 text-green-700 dark:border-green-800 dark:text-green-400 font-medium' : 'font-medium'}
                        >
                          <div className={`w-2 h-2 rounded-full mr-1.5 ${user.status === 'Active' ? 'bg-green-500' : 'bg-muted-foreground'}`} />
                          {user.status}
                        </Badge>
                      </td>
                      <td className="p-3">
                        <Button variant="ghost" size="sm" className="hover:bg-muted transition-colors">
                          Edit
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}