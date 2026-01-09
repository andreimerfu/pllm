import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import api from '@/lib/api';
import { ApiKey } from '@/components/keys/columns';

export function useKeys(isAdmin: boolean) {
  const queryClient = useQueryClient();

  const {
    data: keysData,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['keys', isAdmin],
    queryFn: async () => {
      const data: any = await api.keys.list(!isAdmin);
      return (data.keys || []) as ApiKey[];
    },
  });

  const keys = keysData || [];

  // Generate key mutation
  const generateKeyMutation = useMutation({
    mutationFn: (keyData: any) => api.keys.generate(keyData, !isAdmin),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['keys'] });
    },
  });

  // Revoke key mutation
  const revokeKeyMutation = useMutation({
    mutationFn: (keyId: string) => api.keys.revoke(keyId, { reason: 'Revoked by admin' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['keys'] });
    },
  });

  // Toggle key status mutation
  const toggleKeyStatusMutation = useMutation({
    mutationFn: ({ keyId, isActive }: { keyId: string; isActive: boolean }) =>
      api.keys.update(keyId, { is_active: !isActive }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['keys'] });
    },
  });

  // Delete key mutation
  const deleteKeyMutation = useMutation({
    mutationFn: (keyId: string) => api.keys.delete(keyId, !isAdmin),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['keys'] });
    },
  });

  return {
    keys,
    isLoading,
    error,
    refetch,
    generateKey: generateKeyMutation.mutateAsync,
    revokeKey: revokeKeyMutation.mutateAsync,
    toggleKeyStatus: toggleKeyStatusMutation.mutateAsync,
    deleteKey: deleteKeyMutation.mutateAsync,
    isGenerating: generateKeyMutation.isPending,
    isRevoking: revokeKeyMutation.isPending,
    isToggling: toggleKeyStatusMutation.isPending,
    isDeleting: deleteKeyMutation.isPending,
  };
}

// Separate hook for user teams
export function useUserTeams() {
  const {
    data: teamsData,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['user-teams'],
    queryFn: async () => {
      const data: any = await api.userProfile.getTeams();
      return data.teams || [];
    },
  });

  return {
    teams: teamsData || [],
    isLoading,
    error,
  };
}
