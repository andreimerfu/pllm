import { useQuery } from '@tanstack/react-query';
import api from '@/lib/api';
import { User } from '@/types/api';

export function useUsers() {
  const {
    data: usersData,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['users'],
    queryFn: async () => {
      const response: any = await api.users.list();
      return (response as User[]) || [];
    },
  });

  const users = usersData || [];

  return {
    users,
    isLoading,
    error,
    refetch,
  };
}
