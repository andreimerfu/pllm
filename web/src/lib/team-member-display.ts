import type { TeamMember } from '@/hooks/useTeamMembers';

export function getMemberDisplayName(member: TeamMember): string {
  const user = member.user;
  if (user?.first_name || user?.last_name) {
    return `${user.first_name ?? ''} ${user.last_name ?? ''}`.trim();
  }
  if (user?.username) return user.username;
  if (user?.email) return user.email;
  return 'Unknown user';
}

export function getMemberEmail(member: TeamMember): string {
  return member.user?.email ?? '';
}

export function getMemberInitial(member: TeamMember): string {
  const user = member.user;
  const source =
    user?.first_name || user?.username || user?.email || '';
  return source.charAt(0).toUpperCase() || 'U';
}

export function getMemberAvatarUrl(member: TeamMember): string | undefined {
  return member.user?.avatar_url || undefined;
}
