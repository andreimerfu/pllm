import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api from '@/lib/api';

export interface Team {
  id: string;
  name: string;
  description?: string;
  created_at: string;
  updated_at: string;
  member_count?: number;
}

export interface CreateTeamInput {
  name: string;
  description?: string;
}

export interface UpdateTeamInput {
  name: string;
  description?: string;
}

export function useTeams() {
  const queryClient = useQueryClient();

  // Fetch all teams
  const {
    data: teamsData,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['teams'],
    queryFn: async () => {
      const response: any = await api.teams.list();
      return response.teams || [];
    },
  });

  const teams = (teamsData as Team[]) || [];

  // Create team mutation
  const createTeamMutation = useMutation({
    mutationFn: (data: CreateTeamInput) => api.teams.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['teams'] });
    },
  });

  // Update team mutation
  const updateTeamMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateTeamInput }) =>
      api.teams.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['teams'] });
    },
  });

  // Delete team mutation
  const deleteTeamMutation = useMutation({
    mutationFn: (id: string) => api.teams.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['teams'] });
    },
  });

  return {
    teams,
    isLoading,
    error,
    refetch,
    createTeam: createTeamMutation.mutateAsync,
    updateTeam: (id: string, data: UpdateTeamInput) =>
      updateTeamMutation.mutateAsync({ id, data }),
    deleteTeam: deleteTeamMutation.mutateAsync,
    isCreating: createTeamMutation.isPending,
    isUpdating: updateTeamMutation.isPending,
    isDeleting: deleteTeamMutation.isPending,
  };
}
