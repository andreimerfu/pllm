import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api from '@/lib/api';

export interface TeamMember {
  id: string;
  user_id: string;
  team_id: string;
  role: 'admin' | 'member' | 'viewer';
  email?: string;
  name?: string;
  created_at: string;
  updated_at: string;
}

export interface AddMemberInput {
  user_id: string;
  role: 'admin' | 'member' | 'viewer';
}

export interface UpdateMemberInput {
  role: 'admin' | 'member' | 'viewer';
}

export function useTeamMembers(teamId: string | null) {
  const queryClient = useQueryClient();

  // Fetch team details including members
  const {
    data: teamData,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['team', teamId],
    queryFn: () => (teamId ? api.teams.get(teamId) : null),
    enabled: !!teamId,
  });

  const members = (teamData as any)?.members || [];

  // Add member mutation
  const addMemberMutation = useMutation({
    mutationFn: ({ teamId, data }: { teamId: string; data: AddMemberInput }) =>
      api.teams.addMember(teamId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', teamId] });
      queryClient.invalidateQueries({ queryKey: ['teams'] });
    },
  });

  // Update member mutation
  const updateMemberMutation = useMutation({
    mutationFn: ({
      teamId,
      memberId,
      data,
    }: {
      teamId: string;
      memberId: string;
      data: UpdateMemberInput;
    }) => api.teams.updateMember(teamId, memberId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', teamId] });
      queryClient.invalidateQueries({ queryKey: ['teams'] });
    },
  });

  // Remove member mutation
  const removeMemberMutation = useMutation({
    mutationFn: ({ teamId, memberId }: { teamId: string; memberId: string }) =>
      api.teams.removeMember(teamId, memberId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['team', teamId] });
      queryClient.invalidateQueries({ queryKey: ['teams'] });
    },
  });

  return {
    members,
    isLoading,
    error,
    refetch,
    addMember: (data: AddMemberInput) =>
      teamId ? addMemberMutation.mutateAsync({ teamId, data }) : Promise.reject('No team selected'),
    updateMember: (memberId: string, data: UpdateMemberInput) =>
      teamId
        ? updateMemberMutation.mutateAsync({ teamId, memberId, data })
        : Promise.reject('No team selected'),
    removeMember: (memberId: string) =>
      teamId
        ? removeMemberMutation.mutateAsync({ teamId, memberId })
        : Promise.reject('No team selected'),
    isAdding: addMemberMutation.isPending,
    isUpdating: updateMemberMutation.isPending,
    isRemoving: removeMemberMutation.isPending,
  };
}
