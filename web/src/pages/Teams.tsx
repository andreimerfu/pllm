import React, { useState, useEffect } from 'react';
import { Icon } from '@iconify/react';
import { icons } from '@/lib/icons';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import { useToast } from '../components/ui/use-toast';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../components/ui/dropdown-menu';
import { Avatar, AvatarFallback, AvatarImage } from '../components/ui/avatar';
import { formatDate } from '../lib/date-utils';
import { EmptyState } from '../components/common/EmptyState';
import { LoadingState } from '../components/common/LoadingState';
import { PageShell, StatTile } from '../components/common/page-shell';
import { StatusChip, SectionLabel, type StatusTone } from '../components/common/status-chip';
import { useTeams, type Team } from '../hooks/useTeams';
import { useTeamMembers, type TeamMember } from '../hooks/useTeamMembers';
import { useTeamAnalytics } from '../hooks/useTeamAnalytics';
import { CreateTeamModal } from '../components/teams/CreateTeamModal';
import { EditTeamModal } from '../components/teams/EditTeamModal';
import { AddMemberModal } from '../components/teams/AddMemberModal';
import { EditMemberModal } from '../components/teams/EditMemberModal';
import {
  getMemberDisplayName,
  getMemberInitial,
  getMemberEmail,
  getMemberAvatarUrl,
} from '../lib/team-member-display';

// Map a member role to a StatusChip tone:
// Admin → info, Maintainer → good, everything else (Member/Viewer) → neutral.
const roleTone = (role: string): StatusTone => {
  const r = (role || '').toLowerCase();
  if (r === 'admin') return 'info';
  if (r === 'maintainer') return 'good';
  return 'neutral';
};

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
      <PageShell
        title="Teams"
        description="Groups that scope keys, budgets, and model access."
        actions={
          <Button onClick={() => setShowCreateModal(true)}>
            <Icon icon={icons.plus} className="mr-2 h-4 w-4" />
            Create Team
          </Button>
        }
      >
        <EmptyState
          icon={icons.teams}
          title="No teams"
          description="Create your first team to organize users and manage access."
          className="mt-6"
          action={
            <Button onClick={() => setShowCreateModal(true)}>
              <Icon icon={icons.plus} className="mr-2 h-4 w-4" />
              Create First Team
            </Button>
          }
        />
        <CreateTeamModal open={showCreateModal} onOpenChange={setShowCreateModal} />
      </PageShell>
    );
  }

  return (
    <PageShell
      title="Teams"
      description="Groups that scope keys, budgets, and model access. Membership maps to Dex groups."
      actions={
        <Button onClick={() => setShowCreateModal(true)}>
          <Icon icon={icons.plus} className="mr-2 h-4 w-4" />
          Create Team
        </Button>
      }
    >
      <div className="grid grid-cols-1 md:grid-cols-[280px_1fr] lg:grid-cols-[300px_1fr] gap-6">
        {/* Teams List Sidebar */}
        <div className="md:sticky md:top-4 self-start space-y-2">
          <SectionLabel>Your Teams</SectionLabel>
          <div className="space-y-2">
            {teams.map((team) => {
              const isSelected = selectedTeam?.id === team.id;
              return (
                <div
                  key={team.id}
                  onClick={() => setSelectedTeam(team)}
                  className={`group flex items-center gap-3 rounded-[14px] border p-3 shadow-sm cursor-pointer transition-colors ${
                    isSelected
                      ? 'border-primary/40 bg-[var(--accent-soft)]'
                      : 'border-border bg-card hover:border-border-strong'
                  }`}
                >
                  <span
                    className={`inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px] text-[13px] font-semibold uppercase ${
                      isSelected
                        ? 'bg-primary/15 text-primary'
                        : 'bg-surface-2 text-text-dim'
                    }`}
                  >
                    {team.name?.[0] ?? '?'}
                  </span>
                  <div className="flex-1 min-w-0">
                    <p className="truncate font-medium text-foreground">{team.name}</p>
                    <p className="text-[12px] text-muted-foreground">{team.member_count || 0} members</p>
                  </div>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                      <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                        <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={(e) => {
                        e.stopPropagation();
                        setSelectedTeam(team);
                        setShowEditTeamModal(true);
                      }}>
                        <Icon icon={icons.edit} className="mr-2 h-4 w-4" />
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
                        <Icon icon={icons.delete} className="mr-2 h-4 w-4" />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              );
            })}
          </div>
        </div>

        {/* Team Details */}
        <div className="space-y-6 min-w-0">
          {selectedTeam ? (
            <>
              {/* KPI Row */}
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <StatTile
                  label="Members"
                  value={members.length}
                  iconColor="primary"
                  icon={<Icon icon={icons.teams} />}
                />
                <StatTile
                  label="Requests"
                  value={stats?.total_requests?.toLocaleString() || '0'}
                  iconColor="info"
                  icon={<Icon icon={icons.trendingUp} />}
                />
                <StatTile
                  label="Cost"
                  value={`$${(stats?.total_cost || 0).toFixed(2)}`}
                  tone="good"
                  iconColor="good"
                  icon={<Icon icon={icons.budget} />}
                />
                <StatTile
                  label="Avg / Req"
                  value={`$${((stats?.total_cost || 0) / (stats?.total_requests || 1)).toFixed(4)}`}
                  iconColor="model"
                  icon={<Icon icon={icons.trendingUp} />}
                />
              </div>

              <Tabs defaultValue="members" className="w-full">
                <div className="flex items-center justify-between gap-4">
                  <TabsList>
                    <TabsTrigger value="members">
                      <Icon icon={icons.teams} className="mr-2 h-4 w-4" />
                      Members
                    </TabsTrigger>
                    <TabsTrigger value="analytics">
                      <Icon icon={icons.trendingUp} className="mr-2 h-4 w-4" />
                      Analytics
                    </TabsTrigger>
                    <TabsTrigger value="settings">
                      <Icon icon={icons.settings} className="mr-2 h-4 w-4" />
                      Settings
                    </TabsTrigger>
                  </TabsList>
                  <Button size="sm" onClick={() => setShowAddMemberModal(true)}>
                    <Icon icon={icons.plus} className="mr-2 h-4 w-4" />
                    Add Member
                  </Button>
                </div>

                {/* Members Tab */}
                <TabsContent value="members">
                  {membersLoading ? (
                    <LoadingState text="Loading members..." />
                  ) : members.length === 0 ? (
                    <EmptyState
                      icon={icons.teams}
                      title="No team members"
                      description="Start building your team by adding members with different roles and permissions."
                      action={
                        <Button onClick={() => setShowAddMemberModal(true)}>
                          <Icon icon={icons.plus} className="mr-2 h-4 w-4" />
                          Add First Member
                        </Button>
                      }
                    />
                  ) : (
                    <div className="space-y-2">
                      {members.map((member: TeamMember) => (
                        <div
                          key={member.id}
                          className="flex items-center justify-between gap-3 rounded-[14px] border border-border bg-card p-4 shadow-sm"
                        >
                          <div className="flex min-w-0 items-center gap-3">
                            <Avatar>
                              {getMemberAvatarUrl(member) && (
                                <AvatarImage
                                  src={getMemberAvatarUrl(member)}
                                  alt={getMemberDisplayName(member)}
                                />
                              )}
                              <AvatarFallback>{getMemberInitial(member)}</AvatarFallback>
                            </Avatar>
                            <div className="min-w-0">
                              <p className="truncate font-medium text-foreground">{getMemberDisplayName(member)}</p>
                              <p className="truncate text-[12.5px] text-muted-foreground">
                                {getMemberEmail(member) || '\u2014'}
                              </p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2 shrink-0">
                            <StatusChip tone={roleTone(member.role)} label={member.role} dot={false} />
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="sm">
                                  <Icon icon={icons.moreHorizontal} className="h-4 w-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                <DropdownMenuItem onClick={() => handleEditMember(member)}>
                                  <Icon icon={icons.edit} className="mr-2 h-4 w-4" />
                                  Edit Role
                                </DropdownMenuItem>
                                <DropdownMenuSeparator />
                                <DropdownMenuItem
                                  className="text-destructive"
                                  onClick={() => handleRemoveMember(member.id)}
                                  disabled={isRemoving}
                                >
                                  <Icon icon={icons.delete} className="mr-2 h-4 w-4" />
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
                      icon={icons.trendingUp}
                      title="No usage data"
                      description="Usage analytics will appear here once team members start making requests."
                    />
                  ) : (
                    <div className="space-y-2">
                      {breakdown.map((user) => (
                        <div key={user.user_id} className="rounded-[14px] border border-border bg-card p-4 shadow-sm">
                          <div className="mb-3 flex items-center justify-between gap-3">
                            <div className="min-w-0">
                              <p className="truncate font-medium text-foreground">{user.name || user.email}</p>
                              <p className="truncate text-[12.5px] text-muted-foreground">{user.email}</p>
                            </div>
                            <StatusChip tone="info" label={`$${user.cost.toFixed(2)}`} dot={false} />
                          </div>
                          <div className="grid grid-cols-2 gap-4 text-sm">
                            <div>
                              <SectionLabel>Requests</SectionLabel>
                              <p className="mt-0.5 font-mono tabular-nums text-foreground">{user.requests.toLocaleString()}</p>
                            </div>
                            <div>
                              <SectionLabel>Tokens</SectionLabel>
                              <p className="mt-0.5 font-mono tabular-nums text-foreground">{user.tokens.toLocaleString()}</p>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </TabsContent>

                {/* Settings Tab */}
                <TabsContent value="settings">
                  <Card className="rounded-[14px] shadow-sm">
                    <CardHeader>
                      <CardTitle>Team Settings</CardTitle>
                      <CardDescription>
                        Configure team settings and preferences
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div>
                        <SectionLabel className="mb-2">Team Information</SectionLabel>
                        <div className="divide-y divide-border rounded-[12px] border border-border bg-surface-2/40 text-sm">
                          <div className="flex items-center justify-between gap-3 px-3 py-2">
                            <span className="text-muted-foreground">Name</span>
                            <span className="truncate text-foreground">{selectedTeam.name}</span>
                          </div>
                          <div className="flex items-center justify-between gap-3 px-3 py-2">
                            <span className="text-muted-foreground">Description</span>
                            <span className="truncate text-foreground">{selectedTeam.description || 'N/A'}</span>
                          </div>
                          <div className="flex items-center justify-between gap-3 px-3 py-2">
                            <span className="text-muted-foreground">Created</span>
                            <span className="text-foreground">{formatDate(selectedTeam.created_at)}</span>
                          </div>
                          <div className="flex items-center justify-between gap-3 px-3 py-2">
                            <span className="text-muted-foreground">Updated</span>
                            <span className="text-foreground">{formatDate(selectedTeam.updated_at)}</span>
                          </div>
                        </div>
                      </div>
                      <Button
                        variant="outline"
                        className="w-full"
                        onClick={() => setShowEditTeamModal(true)}
                      >
                        <Icon icon={icons.edit} className="mr-2 h-4 w-4" />
                        Edit Team Settings
                      </Button>
                    </CardContent>
                  </Card>
                </TabsContent>
              </Tabs>
            </>
          ) : (
            <EmptyState
              icon={icons.teams}
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
    </PageShell>
  );
};

export default Teams;
