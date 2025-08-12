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
    <div className="space-y-4 lg:space-y-6">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold">Users</h1>
          <p className="text-sm lg:text-base text-muted-foreground">Manage user accounts and permissions</p>
        </div>
        <Button className="w-full sm:w-auto">
          <Icon icon="lucide:user-plus" width="16" height="16" className="mr-2" />
          Add User
        </Button>
      </div>

      <Card>
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
              <Card key={user.id} className="p-4">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="font-semibold">{user.name}</h3>
                  <Badge variant={user.role === 'Admin' ? 'default' : 'secondary'}>
                    {user.role}
                  </Badge>
                </div>
                <p className="text-sm text-muted-foreground mb-2">{user.email}</p>
                <div className="flex items-center justify-between">
                  <Badge variant={user.status === 'Active' ? 'outline' : 'secondary'}>
                    {user.status}
                  </Badge>
                  <Button variant="ghost" size="sm">Edit</Button>
                </div>
              </Card>
            ))}
          </div>

          {/* Desktop Table View */}
          <div className="hidden sm:block overflow-x-auto">
            <div className="min-w-[600px]">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left p-2 min-w-[120px]">Name</th>
                    <th className="text-left p-2 min-w-[200px]">Email</th>
                    <th className="text-left p-2 min-w-[80px]">Role</th>
                    <th className="text-left p-2 min-w-[80px]">Status</th>
                    <th className="text-left p-2 min-w-[100px]">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredUsers.map(user => (
                    <tr key={user.id} className="border-b hover:bg-muted/50">
                      <td className="p-2 font-medium">{user.name}</td>
                      <td className="p-2 text-muted-foreground">{user.email}</td>
                      <td className="p-2">
                        <Badge variant={user.role === 'Admin' ? 'default' : 'secondary'}>
                          {user.role}
                        </Badge>
                      </td>
                      <td className="p-2">
                        <Badge variant={user.status === 'Active' ? 'outline' : 'secondary'}>
                          {user.status}
                        </Badge>
                      </td>
                      <td className="p-2">
                        <Button variant="ghost" size="sm">Edit</Button>
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