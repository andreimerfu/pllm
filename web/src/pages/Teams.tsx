import React, { useState, useEffect } from 'react';
import { Plus, Users, Trash2, Edit, Key } from 'lucide-react';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Badge } from '../components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import { useToast } from '../components/ui/use-toast';

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

const Teams: React.FC = () => {
  const [teams, setTeams] = useState<Team[]>([]);
  const [selectedTeam, setSelectedTeam] = useState<Team | null>(null);
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showAddMemberModal, setShowAddMemberModal] = useState(false);
  const [showEditTeamModal, setShowEditTeamModal] = useState(false);
  const [editingMember, setEditingMember] = useState<TeamMember | null>(null);
  const { toast } = useToast();

  useEffect(() => {
    fetchTeams();
  }, []);

  useEffect(() => {
    if (selectedTeam) {
      fetchTeamMembers(selectedTeam.id);
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
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-3xl font-bold">Teams</h1>
        <Button onClick={() => setShowCreateModal(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Create Team
        </Button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Teams List */}
        <div className="lg:col-span-1">
          <Card>
            <CardHeader>
              <CardTitle>All Teams</CardTitle>
              <CardDescription>Select a team to view details</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              {teams.map((team) => (
                <div
                  key={team.id}
                  onClick={() => setSelectedTeam(team)}
                  className={`p-3 rounded-lg cursor-pointer transition-colors ${
                    selectedTeam?.id === team.id
                      ? 'bg-primary/10 border border-primary'
                      : 'hover:bg-gray-100 dark:hover:bg-gray-800'
                  }`}
                >
                  <div className="flex justify-between items-start">
                    <div>
                      <h3 className="font-semibold">{team.name}</h3>
                      <p className="text-sm text-gray-500">{team.description}</p>
                    </div>
                    <Badge variant={team.is_active ? 'default' : 'secondary'}>
                      {team.is_active ? 'Active' : 'Inactive'}
                    </Badge>
                  </div>
                  <div className="mt-2">
                    <div className="flex justify-between text-sm">
                      <span>Budget Used</span>
                      <span>{getBudgetPercentage(team).toFixed(1)}%</span>
                    </div>
                    <div className="w-full bg-gray-200 rounded-full h-2 mt-1">
                      <div
                        className={`h-2 rounded-full ${
                          getBudgetPercentage(team) > 80
                            ? 'bg-red-500'
                            : getBudgetPercentage(team) > 50
                            ? 'bg-yellow-500'
                            : 'bg-green-500'
                        }`}
                        style={{ width: `${Math.min(getBudgetPercentage(team), 100)}%` }}
                      />
                    </div>
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>

        {/* Team Details */}
        {selectedTeam && (
          <div className="lg:col-span-2">
            <Card>
              <CardHeader>
                <div className="flex justify-between items-start">
                  <div>
                    <CardTitle>{selectedTeam.name}</CardTitle>
                    <CardDescription>{selectedTeam.description}</CardDescription>
                  </div>
                  <div className="flex space-x-2">
                    <Button 
                      variant="outline" 
                      size="sm" 
                      onClick={() => setShowEditTeamModal(true)}
                    >
                      <Edit className="h-4 w-4" />
                    </Button>
                    <Button 
                      variant="outline" 
                      size="sm"
                      onClick={() => deleteTeam(selectedTeam.id)}
                    >
                      <Trash2 className="h-4 w-4 text-red-500" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <Tabs defaultValue="overview">
                  <TabsList className="grid w-full grid-cols-4">
                    <TabsTrigger value="overview">Overview</TabsTrigger>
                    <TabsTrigger value="members">Members</TabsTrigger>
                    <TabsTrigger value="keys">API Keys</TabsTrigger>
                    <TabsTrigger value="settings">Settings</TabsTrigger>
                  </TabsList>

                  <TabsContent value="overview" className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <Card>
                        <CardHeader className="pb-2">
                          <CardTitle className="text-sm font-medium">Budget Usage</CardTitle>
                        </CardHeader>
                        <CardContent>
                          <div className="text-2xl font-bold">
                            ${selectedTeam.current_spend.toFixed(2)}
                          </div>
                          <p className="text-xs text-muted-foreground">
                            of ${selectedTeam.max_budget.toFixed(2)} ({selectedTeam.budget_duration})
                          </p>
                          <div className="w-full bg-gray-200 rounded-full h-2 mt-2">
                            <div
                              className={`h-2 rounded-full ${
                                getBudgetPercentage(selectedTeam) > 80
                                  ? 'bg-red-500'
                                  : getBudgetPercentage(selectedTeam) > 50
                                  ? 'bg-yellow-500'
                                  : 'bg-green-500'
                              }`}
                              style={{ width: `${Math.min(getBudgetPercentage(selectedTeam), 100)}%` }}
                            />
                          </div>
                        </CardContent>
                      </Card>

                      <Card>
                        <CardHeader className="pb-2">
                          <CardTitle className="text-sm font-medium">Team Stats</CardTitle>
                        </CardHeader>
                        <CardContent>
                          <div className="space-y-1">
                            <div className="flex justify-between">
                              <span className="text-sm">Members:</span>
                              <span className="text-sm font-medium">{selectedTeam.member_count || members.length}</span>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-sm">API Keys:</span>
                              <span className="text-sm font-medium">{selectedTeam.key_count || 0}</span>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-sm">Reset:</span>
                              <span className="text-sm font-medium">
                                {new Date(selectedTeam.budget_reset_at).toLocaleDateString()}
                              </span>
                            </div>
                          </div>
                        </CardContent>
                      </Card>
                    </div>
                  </TabsContent>

                  <TabsContent value="members" className="space-y-4">
                    <div className="flex justify-between items-center">
                      <h3 className="text-lg font-medium">Team Members</h3>
                      <Button size="sm" onClick={() => setShowAddMemberModal(true)}>
                        <Plus className="mr-2 h-4 w-4" />
                        Add Member
                      </Button>
                    </div>
                    <div className="space-y-2">
                      {members.map((member) => (
                        <div key={member.id} className="flex items-center justify-between p-3 border rounded-lg">
                          <div className="flex items-center space-x-3">
                            <div className="w-10 h-10 bg-gray-200 rounded-full flex items-center justify-center">
                              <Users className="h-5 w-5 text-gray-600" />
                            </div>
                            <div>
                              <p className="font-medium">
                                {member.user?.first_name && member.user?.last_name 
                                  ? `${member.user.first_name} ${member.user.last_name}` 
                                  : member.user?.email || member.user?.username || 'Unknown User'}
                              </p>
                              <p className="text-sm text-gray-500">
                                {member.user?.email} • Role: {member.role}
                                {member.joined_at && ` • Joined: ${new Date(member.joined_at).toLocaleDateString()}`}
                              </p>
                            </div>
                          </div>
                          <div className="flex items-center space-x-2">
                            {member.max_budget && (
                              <Badge variant="outline">
                                ${member.current_spend.toFixed(2)} / ${member.max_budget.toFixed(2)}
                              </Badge>
                            )}
                            <Badge variant="outline">{member.role}</Badge>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setEditingMember(member)}
                            >
                              <Edit className="h-4 w-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => removeMember(member.id)}
                            >
                              <Trash2 className="h-4 w-4 text-red-500" />
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  </TabsContent>

                  <TabsContent value="keys" className="space-y-4">
                    <div className="flex justify-between items-center">
                      <h3 className="text-lg font-medium">API Keys</h3>
                      <Button size="sm">
                        <Key className="mr-2 h-4 w-4" />
                        Generate Key
                      </Button>
                    </div>
                    <p className="text-sm text-gray-500">Team API keys will be displayed here</p>
                  </TabsContent>

                  <TabsContent value="settings" className="space-y-4">
                    <h3 className="text-lg font-medium">Team Settings</h3>
                    <div className="space-y-4">
                      <div>
                        <Label>Team Name</Label>
                        <Input id="team-settings-name" defaultValue={selectedTeam.name} />
                      </div>
                      <div>
                        <Label>Description</Label>
                        <Input id="team-settings-description" defaultValue={selectedTeam.description} />
                      </div>
                      <div>
                        <Label>Budget Limit</Label>
                        <Input id="team-settings-budget" type="number" defaultValue={selectedTeam.max_budget} />
                      </div>
                      <div>
                        <Label>Budget Period</Label>
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
                      }}>Save Changes</Button>
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
          <Card className="w-96">
            <CardHeader>
              <CardTitle>Add Team Member</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label>User ID</Label>
                <Input id="member-user-id" placeholder="Enter user ID" />
              </div>
              <div>
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
              <div>
                <Label>Max Budget (Optional)</Label>
                <Input id="member-budget" type="number" placeholder="0.00" />
              </div>
              <div className="flex justify-end space-x-2">
                <Button variant="outline" onClick={() => setShowAddMemberModal(false)}>
                  Cancel
                </Button>
                <Button onClick={() => {
                  const userId = (document.getElementById('member-user-id') as HTMLInputElement)?.value;
                  const role = (document.getElementById('member-role') as HTMLSelectElement)?.value;
                  const budget = parseFloat((document.getElementById('member-budget') as HTMLInputElement)?.value || '0');
                  
                  if (userId) {
                    addMember({
                      user_id: userId,
                      role,
                      max_budget: budget > 0 ? budget : undefined,
                    });
                    setShowAddMemberModal(false);
                  }
                }}>
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