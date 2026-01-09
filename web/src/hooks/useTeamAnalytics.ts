import { useQuery } from '@tanstack/react-query';
import api, { getTeamUserBreakdown } from '@/lib/api';

export interface TeamStats {
  total_requests: number;
  total_tokens: number;
  total_cost: number;
  member_count: number;
}

export interface TeamUserBreakdown {
  user_id: string;
  email: string;
  name?: string;
  requests: number;
  tokens: number;
  cost: number;
}

export function useTeamAnalytics(teamId: string | null) {
  // Fetch team stats
  const {
    data: statsData,
    isLoading: isLoadingStats,
    error: statsError,
    refetch: refetchStats,
  } = useQuery({
    queryKey: ['team-stats', teamId],
    queryFn: async () => {
      if (!teamId) return null;
      return await api.teams.getStats(teamId);
    },
    enabled: !!teamId,
  });

  const stats = statsData as TeamStats | undefined;

  // Fetch team user breakdown
  const {
    data: breakdownData,
    isLoading: isLoadingBreakdown,
    error: breakdownError,
    refetch: refetchBreakdown,
  } = useQuery({
    queryKey: ['team-user-breakdown', teamId],
    queryFn: async () => {
      if (!teamId) return [];
      const response: any = await getTeamUserBreakdown(teamId);
      return response.users || [];
    },
    enabled: !!teamId,
  });

  const breakdown = (breakdownData as TeamUserBreakdown[]) || [];

  return {
    stats,
    breakdown,
    isLoading: isLoadingStats || isLoadingBreakdown,
    error: statsError || breakdownError,
    refetch: () => {
      refetchStats();
      refetchBreakdown();
    },
  };
}
