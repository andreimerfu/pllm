import React, { useState, useEffect } from 'react';
import { 
  Plus, 
  Users, 
  Trash2, 
  Edit, 
  Key, 
  Activity, 
  MoreHorizontal,
  Settings,
  TrendingUp,
  DollarSign,
  Calendar,
  Search,
  Check,
  ChevronsUpDown
} from 'lucide-react';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Badge } from '../components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import { useToast } from '../components/ui/use-toast';
import { 
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../components/ui/dropdown-menu';
import { Progress } from '../components/ui/progress';
import { Avatar, AvatarFallback } from '../components/ui/avatar';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '../components/ui/command';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '../components/ui/popover';
import { cn } from '../lib/utils';
import { getTeamUserBreakdown } from '../lib/api';

interface Team {
  id: string;
  name: string;
  description: string;
  max_budget: number;
  current_spend: number;
  budget_duration: string;
  budget_reset_at: string;
  member_count?: number;
  key_count?: number;
  is_active: boolean;
  created_at: string;
  members?: TeamMember[];
}

interface User {
  id: string;
  email: string;
  username: string;
  first_name: string;
  last_name: string;
  role: string;
  is_active: boolean;
}

interface TeamMember {
  id: string;
  team_id: string;
  user_id: string;
  user?: User;
  role: string;
  max_budget?: number;
  current_spend: number;
  joined_at: string;
}

interface UserUsageStats {
  user_id: string;
  username?: string;
  email?: string;
  total_requests: number;
  total_cost: number;
  avg_cost_per_request: number;
  last_activity: string;
  models_used: string[];
}

const Teams: React.FC = () => {
  const [teams, setTeams] = useState<Team[]>([]);
  const [selectedTeam, setSelectedTeam] = useState<Team | null>(null);
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [userAnalytics, setUserAnalytics] = useState<UserUsageStats[]>([]);
  const [loading, setLoading] = useState(true);
  const [analyticsLoading, setAnalyticsLoading] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showAddMemberModal, setShowAddMemberModal] = useState(false);
  const [showEditTeamModal, setShowEditTeamModal] = useState(false);
  const [editingMember, setEditingMember] = useState<TeamMember | null>(null);
  const [availableUsers, setAvailableUsers] = useState<User[]>([]);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [userSearchOpen, setUserSearchOpen] = useState(false);
  const { toast } = useToast();

  useEffect(() => {
    fetchTeams();
    fetchAvailableUsers();
  }, []);

  useEffect(() => {
    if (selectedTeam) {
      fetchTeamMembers(selectedTeam.id);
      fetchTeamAnalytics(selectedTeam.id);
    }
  }, [selectedTeam]);

  const fetchTeams = async () => {
    try {
      const response = await fetch('/api/admin/teams', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token') || localStorage.getItem('authToken')}`,
        },
      });
      if (response.ok) {
        const data = await response.json();
        setTeams(data.teams || []);
        if (data.teams?.length > 0 && !selectedTeam) {
          setSelectedTeam(data.teams[0]);
        }
      }
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to fetch teams',
        variant: 'destructive',
      });
    } finally {
      setLoading(false);
    }
  };

  const fetchTeamMembers = async (teamId: string) => {
    try {
      const response = await fetch(`/api/admin/teams/${teamId}`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token') || localStorage.getItem('authToken')}`,
        },
      });
      if (response.ok) {
        const teamData = await response.json();
        setMembers(teamData.members || []);
      }
    } catch (error) {
      console.error('Failed to fetch team members:', error);
    }
  };

  const fetchAvailableUsers = async () => {
    try {
      const response = await fetch('/api/admin/users', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token') || localStorage.getItem('authToken')}`,
        },
      });
      if (response.ok) {
        const data = await response.json();
        console.log('Fetched users data:', data);
        const users = Array.isArray(data) ? data : data.users || [];
        console.log('Processed users:', users);
        setAvailableUsers(users);
      }
    } catch (error) {
      console.error('Failed to fetch users:', error);
    }
  };

  const fetchTeamAnalytics = async (teamId: string) => {
    setAnalyticsLoading(true);
    try {
      const data: any = await getTeamUserBreakdown(teamId);
      setUserAnalytics(data?.users || []);
    } catch (error) {
      console.error('Failed to fetch team analytics:', error);
      toast({
        title: 'Warning',
        description: 'Failed to load user analytics',
        variant: 'destructive',
      });
    } finally {
      setAnalyticsLoading(false);
    }
  };

  const createTeam = async (teamData: any) => {
    try {
      const response = await fetch('/api/admin/teams', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token') || localStorage.getItem('authToken')}`,
        },
        body: JSON.stringify(teamData),
      });

      if (response.ok) {
        const newTeam = await response.json();
        setTeams([...teams, newTeam]);
        setShowCreateModal(false);
        toast({
          title: 'Success',
          description: 'Team created successfully',
        });
      }
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to create team',
        variant: 'destructive',
      });
    }
  };

  const updateTeam = async (teamData: any) => {
    if (!selectedTeam) return;

    try {
      const response = await fetch(`/api/admin/teams/${selectedTeam.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token') || localStorage.getItem('authToken')}`,
        },
        body: JSON.stringify(teamData),
      });

      if (response.ok) {
        const updatedTeam = await response.json();
        setTeams(teams.map(t => t.id === updatedTeam.id ? updatedTeam : t));
        setSelectedTeam(updatedTeam);
        toast({
          title: 'Success',
          description: 'Team updated successfully',
        });
      }
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to update team',
        variant: 'destructive',
      });
    }
  };

  const deleteTeam = async (teamId: string) => {
    if (!confirm('Are you sure you want to delete this team? This action cannot be undone.')) return;

    try {
      const response = await fetch(`/api/admin/teams/${teamId}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token') || localStorage.getItem('authToken')}`,
        },
      });

      if (response.ok) {
        setTeams(teams.filter(t => t.id !== teamId));
        if (selectedTeam?.id === teamId) {
          setSelectedTeam(teams.length > 1 ? teams[0] : null);
        }
        toast({
          title: 'Success',
          description: 'Team deleted successfully',
        });
      }
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to delete team',
        variant: 'destructive',
      });
    }
  };

  const addMember = async (memberData: any) => {
    if (!selectedTeam) return;

    try {
      const response = await fetch(`/api/admin/teams/${selectedTeam.id}/members`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token') || localStorage.getItem('authToken')}`,
        },
        body: JSON.stringify(memberData),
      });

      if (response.ok) {
        const newMember = await response.json();
        setMembers([...members, newMember]);
        toast({
          title: 'Success',
          description: 'Member added successfully',
        });
        setShowAddMemberModal(false);
        setSelectedUser(null);
        fetchTeamMembers(selectedTeam.id); // Refresh to get user data
      }
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to add member',
        variant: 'destructive',
      });
    }
  };

  const updateMember = async (memberId: string, memberData: any) => {
    if (!selectedTeam) return;

    try {
      const response = await fetch(`/api/admin/teams/${selectedTeam.id}/members/${memberId}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token') || localStorage.getItem('authToken')}`,
        },
        body: JSON.stringify(memberData),
      });

      if (response.ok) {
        const updatedMember = await response.json();
        setMembers(members.map(m => m.id === memberId ? updatedMember : m));
        toast({
          title: 'Success',
          description: 'Member updated successfully',
        });
        fetchTeamMembers(selectedTeam.id); // Refresh to get user data
      }
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to update member',
        variant: 'destructive',
      });
    }
  };

  const removeMember = async (memberId: string) => {
    if (!selectedTeam) return;

    try {
      const response = await fetch(`/api/admin/teams/${selectedTeam.id}/members/${memberId}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token') || localStorage.getItem('authToken')}`,
        },
      });

      if (response.ok) {
        setMembers(members.filter(m => m.id !== memberId));
        toast({
          title: 'Success',
          description: 'Member removed successfully',
        });
      }
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to remove member',
        variant: 'destructive',
      });
    }
  };

  const getBudgetPercentage = (team: Team) => {
    if (team.max_budget === 0) return 0;
    return (team.current_spend / team.max_budget) * 100;
  };

  if (loading) {
    return <div className="flex items-center justify-center h-96">Loading...</div>;
  }

  return (
    <div className="flex-1 space-y-6 p-6">
      {/* Header */}
      <div className="flex items-center justify-between space-y-2">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Teams</h1>
          <p className="text-muted-foreground">
            Manage team access, budgets, and member permissions
          </p>
        </div>
        <div className="flex items-center space-x-2">
          <div className="relative">
            <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search teams..."
              className="pl-8 w-[200px] lg:w-[250px]"
            />
          </div>
          <Button onClick={() => setShowCreateModal(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create Team
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        {/* Teams Sidebar */}
        <div className="lg:col-span-1">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Teams</CardTitle>
              <CardDescription>Select a team to manage</CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              <div className="space-y-1">
                {teams.map((team) => (
                  <div
                    key={team.id}
                    onClick={() => setSelectedTeam(team)}
                    className={`flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium cursor-pointer transition-colors ${
                      selectedTeam?.id === team.id
                        ? 'bg-secondary text-secondary-foreground'
                        : 'hover:bg-secondary/50'
                    }`}
                  >
                    <Avatar className="h-8 w-8">
                      <AvatarFallback className="text-xs">
                        {team.name.substring(0, 2).toUpperCase()}
                      </AvatarFallback>
                    </Avatar>
                    <div className="flex-1 min-w-0">
                      <div className="font-medium truncate">{team.name}</div>
                      <div className="text-xs text-muted-foreground flex items-center gap-1">
                        <Users className="h-3 w-3" />
                        {team.member_count || members.length} members
                      </div>
                    </div>
                    <Badge 
                      variant={team.is_active ? 'default' : 'secondary'}
                      className="text-xs"
                    >
                      {team.is_active ? 'Active' : 'Inactive'}
                    </Badge>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Team Details */}
        {selectedTeam && (
          <div className="lg:col-span-3">
            {/* Team Header */}
            <div className="flex items-center justify-between space-y-2 mb-6">
              <div className="flex items-center space-x-4">
                <Avatar className="h-12 w-12">
                  <AvatarFallback className="text-lg font-semibold">
                    {selectedTeam.name.substring(0, 2).toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <div>
                  <h2 className="text-2xl font-bold">{selectedTeam.name}</h2>
                  <p className="text-muted-foreground">{selectedTeam.description}</p>
                </div>
              </div>
              <div className="flex items-center space-x-2">
                <Button variant="outline" size="sm" onClick={() => setShowEditTeamModal(true)}>
                  <Settings className="h-4 w-4 mr-2" />
                  Settings
                </Button>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="sm">
                      <MoreHorizontal className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => setShowEditTeamModal(true)}>
                      <Edit className="mr-2 h-4 w-4" />
                      Edit Team
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem 
                      onClick={() => deleteTeam(selectedTeam.id)}
                      className="text-destructive focus:text-destructive"
                    >
                      <Trash2 className="mr-2 h-4 w-4" />
                      Delete Team
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
            
            {/* Quick Stats Cards */}
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Budget Used</CardTitle>
                  <DollarSign className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    ${selectedTeam.current_spend.toFixed(2)}
                  </div>
                  <div className="flex items-center space-x-2 text-xs text-muted-foreground">
                    <span>of ${selectedTeam.max_budget.toFixed(2)}</span>
                  </div>
                  <Progress 
                    value={getBudgetPercentage(selectedTeam)} 
                    className="mt-2" 
                  />
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Team Members</CardTitle>
                  <Users className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {selectedTeam.member_count || members.length}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Active members
                  </p>
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">API Keys</CardTitle>
                  <Key className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {selectedTeam.key_count || 0}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Generated keys
                  </p>
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Budget Reset</CardTitle>
                  <Calendar className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {new Date(selectedTeam.budget_reset_at).getDate()}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {new Date(selectedTeam.budget_reset_at).toLocaleDateString('en-US', { month: 'short' })}
                  </p>
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-lg">Team Management</CardTitle>
                <CardDescription>Manage members, settings, and analytics</CardDescription>
              </CardHeader>
              <CardContent>
                <Tabs defaultValue="members" className="w-full">
                  <TabsList className="grid w-full grid-cols-4">
                    <TabsTrigger value="members">Members</TabsTrigger>
                    <TabsTrigger value="analytics">Analytics</TabsTrigger>
                    <TabsTrigger value="keys">API Keys</TabsTrigger>
                    <TabsTrigger value="settings">Settings</TabsTrigger>
                  </TabsList>

                  <TabsContent value="members" className="space-y-4">
                    <div className="flex justify-between items-center">
                      <div>
                        <h3 className="text-lg font-semibold">Team Members</h3>
                        <p className="text-sm text-muted-foreground">
                          Manage team member access and permissions
                        </p>
                      </div>
                      <Button onClick={() => setShowAddMemberModal(true)}>
                        <Plus className="mr-2 h-4 w-4" />
                        Add Member
                      </Button>
                    </div>
                    
                    {members.length > 0 ? (
                      <div className="space-y-2">
                        {members.map((member) => (
                          <Card key={member.id} className="p-4">
                            <div className="flex items-center justify-between">
                              <div className="flex items-center space-x-4">
                                <Avatar className="h-10 w-10">
                                  <AvatarFallback>
                                    {(member.user?.first_name?.[0] || '') + (member.user?.last_name?.[0] || '') || 
                                     member.user?.email?.[0]?.toUpperCase() || 
                                     member.user?.username?.[0]?.toUpperCase() || 'U'}
                                  </AvatarFallback>
                                </Avatar>
                                <div>
                                  <div className="flex items-center gap-2">
                                    <p className="font-medium">
                                      {member.user?.first_name && member.user?.last_name 
                                        ? `${member.user.first_name} ${member.user.last_name}` 
                                        : member.user?.email || member.user?.username || 'Unknown User'}
                                    </p>
                                    <Badge variant="outline">{member.role}</Badge>
                                  </div>
                                  <div className="flex items-center gap-4 text-sm text-muted-foreground">
                                    <span>{member.user?.email}</span>
                                    {member.joined_at && (
                                      <span>Joined {new Date(member.joined_at).toLocaleDateString()}</span>
                                    )}
                                  </div>
                                </div>
                              </div>
                              <div className="flex items-center space-x-2">
                                {member.max_budget && (
                                  <div className="text-right text-sm">
                                    <div className="font-medium">
                                      ${member.current_spend.toFixed(2)} / ${member.max_budget.toFixed(2)}
                                    </div>
                                    <div className="text-muted-foreground">Budget used</div>
                                  </div>
                                )}
                                <DropdownMenu>
                                  <DropdownMenuTrigger asChild>
                                    <Button variant="ghost" size="sm">
                                      <MoreHorizontal className="h-4 w-4" />
                                    </Button>
                                  </DropdownMenuTrigger>
                                  <DropdownMenuContent align="end">
                                    <DropdownMenuItem onClick={() => setEditingMember(member)}>
                                      <Edit className="mr-2 h-4 w-4" />
                                      Edit Member
                                    </DropdownMenuItem>
                                    <DropdownMenuSeparator />
                                    <DropdownMenuItem 
                                      onClick={() => removeMember(member.id)}
                                      className="text-destructive focus:text-destructive"
                                    >
                                      <Trash2 className="mr-2 h-4 w-4" />
                                      Remove Member
                                    </DropdownMenuItem>
                                  </DropdownMenuContent>
                                </DropdownMenu>
                              </div>
                            </div>
                          </Card>
                        ))}
                      </div>
                    ) : (
                      <Card className="flex flex-col items-center justify-center py-12">
                        <Users className="h-12 w-12 text-muted-foreground mb-4" />
                        <h3 className="text-lg font-semibold mb-2">No team members</h3>
                        <p className="text-muted-foreground text-center mb-4 max-w-sm">
                          Start building your team by adding members with different roles and permissions.
                        </p>
                        <Button onClick={() => setShowAddMemberModal(true)}>
                          <Plus className="mr-2 h-4 w-4" />
                          Add First Member
                        </Button>
                      </Card>
                    )}
                  </TabsContent>

                  <TabsContent value="analytics" className="space-y-6">
                    <div className="flex justify-between items-center">
                      <div>
                        <h3 className="text-lg font-semibold">Team Analytics</h3>
                        <p className="text-sm text-muted-foreground">
                          Monitor team usage, costs, and performance
                        </p>
                      </div>
                      <Button variant="outline" onClick={() => fetchTeamAnalytics(selectedTeam.id)}>
                        <Activity className="mr-2 h-4 w-4" />
                        Refresh Data
                      </Button>
                    </div>
                    
                    {analyticsLoading ? (
                      <div className="flex items-center justify-center py-12">
                        <div className="text-sm text-muted-foreground">Loading analytics data...</div>
                      </div>
                    ) : userAnalytics.length > 0 ? (
                      <div className="space-y-6">
                        {/* Analytics Summary Cards */}
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                          <Card>
                            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                              <CardTitle className="text-sm font-medium">Active Users</CardTitle>
                              <Users className="h-4 w-4 text-muted-foreground" />
                            </CardHeader>
                            <CardContent>
                              <div className="text-2xl font-bold">
                                {userAnalytics.length}
                              </div>
                              <p className="text-xs text-muted-foreground">
                                Team members with activity
                              </p>
                            </CardContent>
                          </Card>
                          <Card>
                            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                              <CardTitle className="text-sm font-medium">Total Requests</CardTitle>
                              <TrendingUp className="h-4 w-4 text-muted-foreground" />
                            </CardHeader>
                            <CardContent>
                              <div className="text-2xl font-bold">
                                {userAnalytics.reduce((sum, user) => sum + user.total_requests, 0).toLocaleString()}
                              </div>
                              <p className="text-xs text-muted-foreground">
                                API calls this period
                              </p>
                            </CardContent>
                          </Card>
                          <Card>
                            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                              <CardTitle className="text-sm font-medium">Total Cost</CardTitle>
                              <DollarSign className="h-4 w-4 text-muted-foreground" />
                            </CardHeader>
                            <CardContent>
                              <div className="text-2xl font-bold">
                                ${userAnalytics.reduce((sum, user) => sum + user.total_cost, 0).toFixed(2)}
                              </div>
                              <p className="text-xs text-muted-foreground">
                                Spent this period
                              </p>
                            </CardContent>
                          </Card>
                        </div>
                        
                        {/* Individual User Analytics */}
                        <div className="space-y-4">
                          <h4 className="font-semibold">Individual User Activity</h4>
                          <div className="space-y-3">
                            {userAnalytics.map((user, index) => (
                              <Card key={user.user_id}>
                                <CardContent className="p-4">
                                  <div className="flex items-center justify-between">
                                    <div className="flex items-center space-x-4">
                                      <Avatar className="h-10 w-10">
                                        <AvatarFallback>
                                          {user.username?.[0]?.toUpperCase() || 
                                           user.email?.[0]?.toUpperCase() || 
                                           'U'}
                                        </AvatarFallback>
                                      </Avatar>
                                      <div>
                                        <p className="font-medium">
                                          {user.username || user.email || `User ${index + 1}`}
                                        </p>
                                        <p className="text-sm text-muted-foreground">
                                          Last active: {new Date(user.last_activity).toLocaleDateString()}
                                        </p>
                                      </div>
                                    </div>
                                    <div className="grid grid-cols-4 gap-6 text-center">
                                      <div>
                                        <div className="text-lg font-semibold">
                                          {user.total_requests.toLocaleString()}
                                        </div>
                                        <div className="text-xs text-muted-foreground">Requests</div>
                                      </div>
                                      <div>
                                        <div className="text-lg font-semibold text-green-600">
                                          ${user.total_cost.toFixed(2)}
                                        </div>
                                        <div className="text-xs text-muted-foreground">Total Cost</div>
                                      </div>
                                      <div>
                                        <div className="text-lg font-semibold text-blue-600">
                                          ${user.avg_cost_per_request.toFixed(4)}
                                        </div>
                                        <div className="text-xs text-muted-foreground">Avg/Request</div>
                                      </div>
                                      <div>
                                        <Badge variant="outline">
                                          {user.models_used.length} models
                                        </Badge>
                                      </div>
                                    </div>
                                  </div>
                                  {user.models_used.length > 0 && (
                                    <div className="mt-4 pt-4 border-t">
                                      <p className="text-sm font-medium mb-2">Models Used:</p>
                                      <div className="flex flex-wrap gap-1">
                                        {user.models_used.slice(0, 5).map((model, i) => (
                                          <Badge key={i} variant="secondary" className="text-xs">
                                            {model}
                                          </Badge>
                                        ))}
                                        {user.models_used.length > 5 && (
                                          <Badge variant="secondary" className="text-xs">
                                            +{user.models_used.length - 5} more
                                          </Badge>
                                        )}
                                      </div>
                                    </div>
                                  )}
                                </CardContent>
                              </Card>
                            ))}
                          </div>
                        </div>
                      </div>
                    ) : (
                      <Card className="flex flex-col items-center justify-center py-12">
                        <Activity className="h-12 w-12 text-muted-foreground mb-4" />
                        <h3 className="text-lg font-semibold mb-2">No activity data</h3>
                        <p className="text-muted-foreground text-center mb-4 max-w-sm">
                          Team analytics will appear here once members start using API keys and making requests.
                        </p>
                        <Button 
                          variant="outline" 
                          onClick={() => fetchTeamAnalytics(selectedTeam.id)}
                        >
                          <Activity className="mr-2 h-4 w-4" />
                          Check Again
                        </Button>
                      </Card>
                    )}
                  </TabsContent>

                  <TabsContent value="keys" className="space-y-6">
                    <div className="flex justify-between items-center">
                      <div>
                        <h3 className="text-lg font-semibold">API Keys</h3>
                        <p className="text-sm text-muted-foreground">
                          Generate and manage API keys for team access
                        </p>
                      </div>
                      <Button>
                        <Key className="mr-2 h-4 w-4" />
                        Generate New Key
                      </Button>
                    </div>
                    
                    <Card className="flex flex-col items-center justify-center py-12">
                      <Key className="h-12 w-12 text-muted-foreground mb-4" />
                      <h3 className="text-lg font-semibold mb-2">No API keys generated</h3>
                      <p className="text-muted-foreground text-center mb-4 max-w-sm">
                        Create API keys to allow team members to access the service programmatically.
                      </p>
                      <Button>
                        <Key className="mr-2 h-4 w-4" />
                        Generate First Key
                      </Button>
                    </Card>
                  </TabsContent>

                  <TabsContent value="settings" className="space-y-6">
                    <div>
                      <h3 className="text-lg font-semibold">Team Settings</h3>
                      <p className="text-sm text-muted-foreground">
                        Configure team details and budget limits
                      </p>
                    </div>
                    
                    <div className="grid gap-6">
                      {/* Basic Information */}
                      <Card>
                        <CardHeader>
                          <CardTitle className="text-base">Basic Information</CardTitle>
                          <CardDescription>
                            Update team name and description
                          </CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                          <div className="grid gap-2">
                            <Label htmlFor="team-settings-name">Team Name</Label>
                            <Input 
                              id="team-settings-name" 
                              placeholder="Enter team name"
                              defaultValue={selectedTeam.name} 
                            />
                          </div>
                          <div className="grid gap-2">
                            <Label htmlFor="team-settings-description">Description</Label>
                            <Input 
                              id="team-settings-description" 
                              placeholder="Enter team description"
                              defaultValue={selectedTeam.description} 
                            />
                          </div>
                        </CardContent>
                      </Card>
                      
                      {/* Budget Settings */}
                      <Card>
                        <CardHeader>
                          <CardTitle className="text-base">Budget Management</CardTitle>
                          <CardDescription>
                            Set spending limits and reset periods
                          </CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                          <div className="grid gap-2">
                            <Label htmlFor="team-settings-budget">Budget Limit ($)</Label>
                            <Input 
                              id="team-settings-budget" 
                              type="number" 
                              step="0.01"
                              min="0"
                              placeholder="0.00"
                              defaultValue={selectedTeam.max_budget} 
                            />
                          </div>
                          <div className="grid gap-2">
                            <Label htmlFor="team-settings-duration">Budget Period</Label>
                            <Select defaultValue={selectedTeam.budget_duration}>
                              <SelectTrigger id="team-settings-duration">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="daily">Daily</SelectItem>
                                <SelectItem value="weekly">Weekly</SelectItem>
                                <SelectItem value="monthly">Monthly</SelectItem>
                                <SelectItem value="yearly">Yearly</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="pt-2">
                            <div className="flex items-center justify-between text-sm">
                              <span className="text-muted-foreground">Next budget reset:</span>
                              <span className="font-medium">
                                {new Date(selectedTeam.budget_reset_at).toLocaleDateString()}
                              </span>
                            </div>
                          </div>
                        </CardContent>
                      </Card>
                      
                      {/* Actions */}
                      <div className="flex justify-between">
                        <Button 
                          variant="destructive" 
                          onClick={() => deleteTeam(selectedTeam.id)}
                        >
                          <Trash2 className="mr-2 h-4 w-4" />
                          Delete Team
                        </Button>
                        <Button onClick={() => {
                          const name = (document.getElementById('team-settings-name') as HTMLInputElement)?.value;
                          const description = (document.getElementById('team-settings-description') as HTMLInputElement)?.value;
                          const budget = parseFloat((document.getElementById('team-settings-budget') as HTMLInputElement)?.value || '0');
                          const duration = (document.getElementById('team-settings-duration') as HTMLSelectElement)?.value;
                          
                          updateTeam({
                            name,
                            description,
                            max_budget: budget,
                            budget_duration: duration,
                          });
                        }}>
                          Save Changes
                        </Button>
                      </div>
                    </div>
                  </TabsContent>
                </Tabs>
              </CardContent>
            </Card>
          </div>
        )}
      </div>

      {/* Create Team Modal - Simplified for now */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-96">
            <CardHeader>
              <CardTitle>Create New Team</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label>Team Name</Label>
                <Input id="team-name" placeholder="Enter team name" />
              </div>
              <div>
                <Label>Description</Label>
                <Input id="team-description" placeholder="Enter description" />
              </div>
              <div>
                <Label>Budget Limit ($)</Label>
                <Input id="team-budget" type="number" placeholder="100.00" />
              </div>
              <div>
                <Label>Budget Period</Label>
                <Select defaultValue="monthly">
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="daily">Daily</SelectItem>
                    <SelectItem value="weekly">Weekly</SelectItem>
                    <SelectItem value="monthly">Monthly</SelectItem>
                    <SelectItem value="yearly">Yearly</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="flex justify-end space-x-2">
                <Button variant="outline" onClick={() => setShowCreateModal(false)}>
                  Cancel
                </Button>
                <Button onClick={() => {
                  // Collect form data and create team
                  const name = (document.getElementById('team-name') as HTMLInputElement)?.value;
                  const description = (document.getElementById('team-description') as HTMLInputElement)?.value;
                  const budget = parseFloat((document.getElementById('team-budget') as HTMLInputElement)?.value || '0');
                  
                  createTeam({
                    name,
                    description,
                    max_budget: budget,
                    budget_duration: 'monthly',
                  });
                }}>
                  Create Team
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Add Member Modal */}
      {showAddMemberModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-[500px]">
            <CardHeader>
              <CardTitle>Add Team Member</CardTitle>
              <CardDescription>
                Select a user from the list and assign them a role in the team
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label>Select User</Label>
                <Popover open={userSearchOpen} onOpenChange={setUserSearchOpen}>
                  <PopoverTrigger asChild>
                    <Button
                      variant="outline"
                      role="combobox"
                      aria-expanded={userSearchOpen}
                      className="w-full justify-between"
                    >
                      {selectedUser ? (
                        <div className="flex items-center gap-2">
                          <Avatar className="h-6 w-6">
                            <AvatarFallback className="text-xs">
                              {((selectedUser.first_name?.[0] || '') + (selectedUser.last_name?.[0] || '')) || 
                               selectedUser.email?.[0]?.toUpperCase() || 
                               selectedUser.username?.[0]?.toUpperCase() || 'U'}
                            </AvatarFallback>
                          </Avatar>
                          <div className="text-left">
                            <div className="text-sm font-medium">
                              {selectedUser.first_name && selectedUser.last_name 
                                ? `${selectedUser.first_name} ${selectedUser.last_name}` 
                                : selectedUser.username || selectedUser.email}
                            </div>
                            <div className="text-xs text-muted-foreground">
                              {selectedUser.email}
                            </div>
                          </div>
                        </div>
                      ) : (
                        "Select user..."
                      )}
                      <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent className="w-[460px] p-0">
                    <Command>
                      <CommandInput placeholder="Search users..." />
                      <CommandList>
                        <CommandEmpty>No users found.</CommandEmpty>
                        <CommandGroup>
                          {availableUsers
                            .filter(user => !members.some(member => member.user_id === user.id))
                            .map((user) => (
                            <CommandItem
                              key={user.id}
                              value={`${user.first_name} ${user.last_name} ${user.username} ${user.email}`}
                              onSelect={() => {
                                setSelectedUser(user);
                                setUserSearchOpen(false);
                              }}
                            >
                              <Check
                                className={cn(
                                  "mr-2 h-4 w-4",
                                  selectedUser?.id === user.id ? "opacity-100" : "opacity-0"
                                )}
                              />
                              <div className="flex items-center gap-2">
                                <Avatar className="h-6 w-6">
                                  <AvatarFallback className="text-xs">
                                    {((user.first_name?.[0] || '') + (user.last_name?.[0] || '')) || 
                                     user.email?.[0]?.toUpperCase() || 
                                     user.username?.[0]?.toUpperCase() || 'U'}
                                  </AvatarFallback>
                                </Avatar>
                                <div>
                                  <div className="text-sm font-medium">
                                    {user.first_name && user.last_name 
                                      ? `${user.first_name} ${user.last_name}` 
                                      : user.username || user.email}
                                  </div>
                                  <div className="text-xs text-muted-foreground">
                                    {user.email} • {user.role}
                                  </div>
                                </div>
                              </div>
                            </CommandItem>
                          ))}
                        </CommandGroup>
                      </CommandList>
                    </Command>
                  </PopoverContent>
                </Popover>
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Select defaultValue="member">
                  <SelectTrigger id="member-role">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="viewer">Viewer</SelectItem>
                    <SelectItem value="member">Member</SelectItem>
                    <SelectItem value="admin">Admin</SelectItem>
                    <SelectItem value="owner">Owner</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Max Budget (Optional)</Label>
                <Input id="member-budget" type="number" step="0.01" min="0" placeholder="0.00" />
                <p className="text-xs text-muted-foreground">
                  Leave empty for unlimited budget within team limits
                </p>
              </div>
              <div className="flex justify-end space-x-2">
                <Button 
                  variant="outline" 
                  onClick={() => {
                    setShowAddMemberModal(false);
                    setSelectedUser(null);
                  }}
                >
                  Cancel
                </Button>
                <Button 
                  onClick={() => {
                    if (!selectedUser) {
                      toast({
                        title: 'Error',
                        description: 'Please select a user',
                        variant: 'destructive',
                      });
                      return;
                    }
                    
                    const role = (document.getElementById('member-role') as HTMLSelectElement)?.value;
                    const budget = parseFloat((document.getElementById('member-budget') as HTMLInputElement)?.value || '0');
                    
                    addMember({
                      user_id: selectedUser.id,
                      role,
                      max_budget: budget > 0 ? budget : undefined,
                    });
                  }}
                  disabled={!selectedUser}
                >
                  Add Member
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Edit Member Modal */}
      {editingMember && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-96">
            <CardHeader>
              <CardTitle>Edit Member</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label>User</Label>
                <Input value={editingMember.user?.email || editingMember.user?.username || 'Unknown'} disabled />
              </div>
              <div>
                <Label>Role</Label>
                <Select defaultValue={editingMember.role}>
                  <SelectTrigger id="edit-member-role">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="viewer">Viewer</SelectItem>
                    <SelectItem value="member">Member</SelectItem>
                    <SelectItem value="admin">Admin</SelectItem>
                    <SelectItem value="owner">Owner</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label>Max Budget (Optional)</Label>
                <Input 
                  id="edit-member-budget" 
                  type="number" 
                  defaultValue={editingMember.max_budget || ''} 
                  placeholder="0.00" 
                />
              </div>
              <div className="flex justify-end space-x-2">
                <Button variant="outline" onClick={() => setEditingMember(null)}>
                  Cancel
                </Button>
                <Button onClick={() => {
                  const role = (document.getElementById('edit-member-role') as HTMLSelectElement)?.value;
                  const budget = parseFloat((document.getElementById('edit-member-budget') as HTMLInputElement)?.value || '0');
                  
                  updateMember(editingMember.id, {
                    role,
                    max_budget: budget > 0 ? budget : undefined,
                  });
                  setEditingMember(null);
                }}>
                  Update Member
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Edit Team Modal */}
      {showEditTeamModal && selectedTeam && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-96">
            <CardHeader>
              <CardTitle>Edit Team</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label>Team Name</Label>
                <Input id="edit-team-name" defaultValue={selectedTeam.name} />
              </div>
              <div>
                <Label>Description</Label>
                <Input id="edit-team-description" defaultValue={selectedTeam.description} />
              </div>
              <div>
                <Label>Budget Limit ($)</Label>
                <Input id="edit-team-budget" type="number" defaultValue={selectedTeam.max_budget} />
              </div>
              <div>
                <Label>Budget Period</Label>
                <Select defaultValue={selectedTeam.budget_duration}>
                  <SelectTrigger id="edit-team-duration">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="daily">Daily</SelectItem>
                    <SelectItem value="weekly">Weekly</SelectItem>
                    <SelectItem value="monthly">Monthly</SelectItem>
                    <SelectItem value="yearly">Yearly</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="flex justify-end space-x-2">
                <Button variant="outline" onClick={() => setShowEditTeamModal(false)}>
                  Cancel
                </Button>
                <Button onClick={() => {
                  const name = (document.getElementById('edit-team-name') as HTMLInputElement)?.value;
                  const description = (document.getElementById('edit-team-description') as HTMLInputElement)?.value;
                  const budget = parseFloat((document.getElementById('edit-team-budget') as HTMLInputElement)?.value || '0');
                  const duration = (document.getElementById('edit-team-duration') as HTMLSelectElement)?.value;
                  
                  updateTeam({
                    name,
                    description,
                    max_budget: budget,
                    budget_duration: duration,
                  });
                  setShowEditTeamModal(false);
                }}>
                  Update Team
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
};

export default Teams;