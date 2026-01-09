import { z } from 'zod';

export const createTeamSchema = z.object({
  name: z.string().min(1, 'Team name is required'),
  description: z.string().optional(),
});

export const updateTeamSchema = z.object({
  name: z.string().min(1, 'Team name is required'),
  description: z.string().optional(),
});

export const addMemberSchema = z.object({
  user_id: z.string().min(1, 'User ID is required'),
  role: z.enum(['admin', 'member', 'viewer'], {
    required_error: 'Role is required',
  }),
});

export const updateMemberSchema = z.object({
  role: z.enum(['admin', 'member', 'viewer'], {
    required_error: 'Role is required',
  }),
});

export type CreateTeamFormData = z.infer<typeof createTeamSchema>;
export type UpdateTeamFormData = z.infer<typeof updateTeamSchema>;
export type AddMemberFormData = z.infer<typeof addMemberSchema>;
export type UpdateMemberFormData = z.infer<typeof updateMemberSchema>;
