import React, { useState, useEffect } from 'react';
import {
  Plus,
  Users,
  Trash2,
  Edit,
  Activity,
  MoreHorizontal,
  Settings,
  TrendingUp,
  DollarSign,
} from 'lucide-react';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import { useToast } from '../components/ui/use-toast';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../components/ui/dropdown-menu';
import { Avatar, AvatarFallback } from '../components/ui/avatar';
import { formatDate } from '../lib/date-utils';
import { EmptyState } from '../components/common/EmptyState';
import { LoadingState } from '../components/common/LoadingState';
import { PageHeader } from '../components/common/PageHeader';
import { useTeams, type Team } from '../hooks/useTeams';
import { useTeamMembers, type TeamMember } from '../hooks/useTeamMembers';
import { useTeamAnalytics } from '../hooks/useTeamAnalytics';
import { CreateTeamModal } from '../components/teams/CreateTeamModal';
import { EditTeamModal } from '../components/teams/EditTeamModal';
import { AddMemberModal } from '../components/teams/AddMemberModal';
import { EditMemberModal } from '../components/teams/EditMemberModal';

const Teams: React.FC = () => {
  const [selectedTeam, setSelectedTeam] = useState<Team | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditTeamModal, setShowEditTeamModal] = useState(false);
  const [showAddMemberModal, setShowAddMemberModal] = useState(false);
  const [showEditMemberModal, setShowEditMemberModal] = useState(false);
  const [editingMember, setEditingMember] = useState<TeamMember | null>(null);
  const { toast } = useToast();

  const { teams, isLoading, deleteTeam, isDeleting } = useTeams();
  const { members, isLoading: membersLoading, removeMember, isRemoving } = useTeamMembers(selectedTeam?.id || null);
  const { stats, breakdown, isLoading: analyticsLoading } = useTeamAnalytics(selectedTeam?.id || null);

  // Set first team as selected on initial load
  useEffect(() => {
    if (teams.length > 0 && !selectedTeam) {
      setSelectedTeam(teams[0]);
    }
  }, [teams, selectedTeam]);

  const handleDeleteTeam = async (teamId: string) => {
    if (!confirm('Are you sure you want to delete this team? This action cannot be undone.')) return;

    try {
      await deleteTeam(teamId);
      toast({
        title: 'Success',
        description: 'Team deleted successfully',
      });
      if (selectedTeam?.id === teamId) {
        setSelectedTeam(teams.length > 1 ? teams[0] : null);
      }
    } catch (error: any) {
      toast({
        title: 'Error',
        description: error.message || 'Failed to delete team',
        variant: 'destructive',
      });
    }
  };

  const handleRemoveMember = async (memberId: string) => {
    if (!confirm('Are you sure you want to remove this member from the team?')) return;

    try {
      await removeMember(memberId);
      toast({
        title: 'Success',
        description: 'Member removed successfully',
      });
    } catch (error: any) {
      toast({
        title: 'Error',
        description: error.message || 'Failed to remove member',
        variant: 'destructive',
      });
    }
  };

  const handleEditMember = (member: TeamMember) => {
    setEditingMember(member);
    setShowEditMemberModal(true);
  };

  if (isLoading) {
    return <LoadingState text="Loading teams..." />;
  }

  if (teams.length === 0) {
    return (
      <>
        <PageHeader
          title="Teams"
          description="Manage teams and their members"
          actions={
            <Button onClick={() => setShowCreateModal(true)}>
              <Plus className="mr-2 h-4 w-4" />
              Create Team
            </Button>
          }
        />
        <EmptyState
          icon={Users}
          title="No teams"
          description="Create your first team to organize users and manage access."
          className="mt-6"
          action={
            <Button onClick={() => setShowCreateModal(true)}>
              <Plus className="mr-2 h-4 w-4" />
              Create First Team
            </Button>
          }
        />
        <CreateTeamModal open={showCreateModal} onOpenChange={setShowCreateModal} />
      </>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Teams"
        description="Manage teams and their members"
        actions={
          <Button onClick={() => setShowCreateModal(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Create Team
          </Button>
        }
      />

      <div className="grid grid-cols-1 md:grid-cols-[280px_1fr] lg:grid-cols-[300px_1fr] gap-6">
        {/* Teams List Sidebar */}
        <Card className="md:sticky md:top-4 self-start">
          <CardHeader className="p-4 pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Your Teams</CardTitle>
          </CardHeader>
          <CardContent className="p-2 space-y-1">
            {teams.map((team) => (
              <div
                key={team.id}
                onClick={() => setSelectedTeam(team)}
                className={`p-3 rounded-lg cursor-pointer transition-colors ${
                  selectedTeam?.id === team.id
                    ? 'bg-primary text-primary-foreground'
                    : 'hover:bg-muted'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div className="flex-1 min-w-0">
                    <p className="font-medium truncate">{team.name}</p>
                    <p className="text-sm opacity-80">{team.member_count || 0} members</p>
                  </div>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                      <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={(e) => {
                        e.stopPropagation();
                        setSelectedTeam(team);
                        setShowEditTeamModal(true);
                      }}>
                        <Edit className="mr-2 h-4 w-4" />
                        Edit
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        className="text-destructive"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDeleteTeam(team.id);
                        }}
                        disabled={isDeleting}
                      >
                        <Trash2 className="mr-2 h-4 w-4" />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>

        {/* Team Details */}
        <div className="space-y-6 min-w-0">
          {selectedTeam ? (
            <>
              {/* Compact Stats Row */}
              <div className="flex flex-wrap items-center gap-x-6 gap-y-3 rounded-lg border bg-card p-4">
                <div className="flex items-center gap-2">
                  <Users className="h-4 w-4 text-muted-foreground" />
                  <span className="text-2xl font-bold tabular-nums">{members.length}</span>
                  <span className="text-sm text-muted-foreground">Members</span>
                </div>
                <div className="hidden sm:block h-8 w-px bg-border" />
                <div className="flex items-center gap-2">
                  <Activity className="h-4 w-4 text-muted-foreground" />
                  <span className="text-2xl font-bold tabular-nums">{stats?.total_requests?.toLocaleString() || '0'}</span>
                  <span className="text-sm text-muted-foreground">Requests</span>
                </div>
                <div className="hidden sm:block h-8 w-px bg-border" />
                <div className="flex items-center gap-2">
                  <DollarSign className="h-4 w-4 text-muted-foreground" />
                  <span className="text-2xl font-bold tabular-nums">${(stats?.total_cost || 0).toFixed(2)}</span>
                  <span className="text-sm text-muted-foreground">Cost</span>
                </div>
                <div className="hidden lg:block h-8 w-px bg-border" />
                <div className="hidden lg:flex items-center gap-2">
                  <TrendingUp className="h-4 w-4 text-muted-foreground" />
                  <span className="text-2xl font-bold tabular-nums">${((stats?.total_cost || 0) / (stats?.total_requests || 1)).toFixed(4)}</span>
                  <span className="text-sm text-muted-foreground">Avg/Req</span>
                </div>
              </div>

              <Tabs defaultValue="members" className="w-full">
                <div className="flex items-center justify-between gap-4">
                  <TabsList>
                    <TabsTrigger value="members">
                      <Users className="mr-2 h-4 w-4" />
                      Members
                    </TabsTrigger>
                    <TabsTrigger value="analytics">
                      <Activity className="mr-2 h-4 w-4" />
                      Analytics
                    </TabsTrigger>
                    <TabsTrigger value="settings">
                      <Settings className="mr-2 h-4 w-4" />
                      Settings
                    </TabsTrigger>
                  </TabsList>
                  <Button size="sm" onClick={() => setShowAddMemberModal(true)}>
                    <Plus className="mr-2 h-4 w-4" />
                    Add Member
                  </Button>
                </div>

                {/* Members Tab */}
                <TabsContent value="members">
                  {membersLoading ? (
                    <LoadingState text="Loading members..." />
                  ) : members.length === 0 ? (
                    <EmptyState
                      icon={Users}
                      title="No team members"
                      description="Start building your team by adding members with different roles and permissions."
                      action={
                        <Button onClick={() => setShowAddMemberModal(true)}>
                          <Plus className="mr-2 h-4 w-4" />
                          Add First Member
                        </Button>
                      }
                    />
                  ) : (
                    <div className="space-y-2">
                      {members.map((member: TeamMember) => (
                        <div
                          key={member.id}
                          className="flex items-center justify-between p-4 border rounded-lg bg-card"
                        >
                          <div className="flex items-center gap-4">
                            <Avatar>
                              <AvatarFallback>
                                {member.email?.[0]?.toUpperCase() || 'U'}
                              </AvatarFallback>
                            </Avatar>
                            <div>
                              <p className="font-medium">{member.name || member.email}</p>
                              <p className="text-sm text-muted-foreground">{member.email}</p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                            <Badge variant={member.role === 'admin' ? 'default' : 'secondary'}>
                              {member.role}
                            </Badge>
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="sm">
                                  <MoreHorizontal className="h-4 w-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                <DropdownMenuItem onClick={() => handleEditMember(member)}>
                                  <Edit className="mr-2 h-4 w-4" />
                                  Edit Role
                                </DropdownMenuItem>
                                <DropdownMenuSeparator />
                                <DropdownMenuItem
                                  className="text-destructive"
                                  onClick={() => handleRemoveMember(member.id)}
                                  disabled={isRemoving}
                                >
                                  <Trash2 className="mr-2 h-4 w-4" />
                                  Remove
                                </DropdownMenuItem>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </TabsContent>

                {/* Analytics Tab */}
                <TabsContent value="analytics">
                  {analyticsLoading ? (
                    <LoadingState text="Loading analytics..." />
                  ) : breakdown.length === 0 ? (
                    <EmptyState
                      icon={Activity}
                      title="No usage data"
                      description="Usage analytics will appear here once team members start making requests."
                    />
                  ) : (
                    <div className="space-y-3">
                      {breakdown.map((user) => (
                        <div key={user.user_id} className="p-4 border rounded-lg bg-card">
                          <div className="flex items-center justify-between mb-2">
                            <div>
                              <p className="font-medium">{user.name || user.email}</p>
                              <p className="text-sm text-muted-foreground">{user.email}</p>
                            </div>
                            <Badge>${user.cost.toFixed(2)}</Badge>
                          </div>
                          <div className="grid grid-cols-2 gap-4 text-sm">
                            <div>
                              <p className="text-muted-foreground">Requests</p>
                              <p className="font-medium">{user.requests.toLocaleString()}</p>
                            </div>
                            <div>
                              <p className="text-muted-foreground">Tokens</p>
                              <p className="font-medium">{user.tokens.toLocaleString()}</p>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </TabsContent>

                {/* Settings Tab */}
                <TabsContent value="settings">
                  <Card>
                    <CardHeader>
                      <CardTitle>Team Settings</CardTitle>
                      <CardDescription>
                        Configure team settings and preferences
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div>
                        <h3 className="font-medium mb-2">Team Information</h3>
                        <div className="space-y-2 text-sm">
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">Name:</span>
                            <span>{selectedTeam.name}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">Description:</span>
                            <span>{selectedTeam.description || 'N/A'}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">Created:</span>
                            <span>{formatDate(selectedTeam.created_at)}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-muted-foreground">Updated:</span>
                            <span>{formatDate(selectedTeam.updated_at)}</span>
                          </div>
                        </div>
                      </div>
                      <Button
                        variant="outline"
                        className="w-full"
                        onClick={() => setShowEditTeamModal(true)}
                      >
                        <Edit className="mr-2 h-4 w-4" />
                        Edit Team Settings
                      </Button>
                    </CardContent>
                  </Card>
                </TabsContent>
              </Tabs>
            </>
          ) : (
            <EmptyState
              icon={Users}
              title="No team selected"
              description="Select a team from the sidebar to view details"
            />
          )}
        </div>
      </div>

      {/* Modals */}
      <CreateTeamModal open={showCreateModal} onOpenChange={setShowCreateModal} />
      <EditTeamModal
        open={showEditTeamModal}
        onOpenChange={setShowEditTeamModal}
        team={selectedTeam}
      />
      <AddMemberModal
        open={showAddMemberModal}
        onOpenChange={setShowAddMemberModal}
        teamId={selectedTeam?.id || null}
      />
      <EditMemberModal
        open={showEditMemberModal}
        onOpenChange={setShowEditMemberModal}
        teamId={selectedTeam?.id || null}
        member={editingMember}
      />
    </div>
  );
};

export default Teams;
