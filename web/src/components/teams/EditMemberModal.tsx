import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useEffect } from 'react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useTeamMembers, type TeamMember } from '@/hooks/useTeamMembers';
import { updateMemberSchema, type UpdateMemberFormData } from '@/lib/schemas/team-schemas';
import { useToast } from '@/hooks/use-toast';

interface EditMemberModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  teamId: string | null;
  member: TeamMember | null;
}

export function EditMemberModal({ open, onOpenChange, teamId, member }: EditMemberModalProps) {
  const { updateMember, isUpdating } = useTeamMembers(teamId);
  const { toast } = useToast();

  const {
    handleSubmit,
    formState: { errors },
    reset,
    setValue,
    watch,
  } = useForm<UpdateMemberFormData>({
    resolver: zodResolver(updateMemberSchema),
  });

  const selectedRole = watch('role');

  useEffect(() => {
    if (member) {
      reset({
        role: member.role,
      });
    }
  }, [member, reset]);

  const onSubmit = async (data: UpdateMemberFormData) => {
    if (!member) return;

    try {
      await updateMember(member.id, data);
      toast({
        title: 'Success',
        description: 'Member role updated successfully',
      });
      onOpenChange(false);
    } catch (error: any) {
      toast({
        title: 'Error',
        description: error.message || 'Failed to update member',
        variant: 'destructive',
      });
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit Member Role</DialogTitle>
          <DialogDescription>
            Update the role for {member?.name || member?.email}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)}>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="role">Role</Label>
              <Select
                value={selectedRole}
                onValueChange={(value) => setValue('role', value as 'admin' | 'member' | 'viewer')}
              >
                <SelectTrigger>
                  <SelectValue placeholder="Select a role" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="admin">Admin</SelectItem>
                  <SelectItem value="member">Member</SelectItem>
                  <SelectItem value="viewer">Viewer</SelectItem>
                </SelectContent>
              </Select>
              {errors.role && (
                <p className="text-sm text-destructive">{errors.role.message}</p>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isUpdating}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isUpdating}>
              {isUpdating ? 'Saving...' : 'Save Changes'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
