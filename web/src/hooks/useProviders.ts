import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getProviderProfiles,
  createProviderProfile,
  updateProviderProfile,
  deleteProviderProfile,
} from '@/lib/api';

export function useProviderProfiles() {
  return useQuery({
    queryKey: ['provider-profiles'],
    queryFn: getProviderProfiles,
  });
}

export function useCreateProviderProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createProviderProfile,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['provider-profiles'] });
    },
  });
}

export function useUpdateProviderProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => updateProviderProfile(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['provider-profiles'] });
    },
  });
}

export function useDeleteProviderProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteProviderProfile,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['provider-profiles'] });
    },
  });
}
