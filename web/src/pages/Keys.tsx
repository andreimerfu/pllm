import React from 'react';
import { useToast } from '../components/ui/use-toast';
import { usePermissions } from '../contexts/PermissionContext';
import { DataTable } from '../components/keys/data-table';
import { createColumns, ApiKey } from '../components/keys/columns';
import { CreateKeyDialog } from '../components/keys/create-key-dialog';
import { LoadingState } from '../components/common/LoadingState';
import { PageHeader } from '../components/common/PageHeader';
import { StatCard } from '../components/common/StatCard';
import { Key, Activity, DollarSign, CheckCircle } from 'lucide-react';
import { useKeys, useUserTeams } from '../hooks/useKeys';

const Keys: React.FC = () => {
  const { toast } = useToast();
  const { hasRole, hasPermission } = usePermissions();

  // Check if user is admin
  const isAdmin = hasRole('admin') || hasPermission('admin.*');

  const { keys, isLoading, generateKey: generateKeyMutation, revokeKey: revokeKeyMutation, toggleKeyStatus: toggleKeyStatusMutation, deleteKey: deleteKeyMutation } = useKeys(isAdmin);
  const { teams: userTeams } = useUserTeams();

  const handleGenerateKey = async (keyData: any) => {
    try {
      const data: any = await generateKeyMutation(keyData);
      
      // Show the new key in a toast
      toast({
        title: 'Key Generated Successfully',
        description: (
          <div className="mt-2">
            <p className="mb-2">Save this key - it won't be shown again!</p>
            <code className="block p-2 bg-muted dark:bg-muted/50 dark:text-foreground rounded text-xs break-all border dark:border-border">
              {data.key || data.plaintext_key || data.PlaintextKey}
            </code>
            <button
              className="mt-2 px-2 py-1 text-xs bg-secondary hover:bg-secondary/80 rounded"
              onClick={() => navigator.clipboard.writeText(data.key || data.plaintext_key || data.PlaintextKey)}
            >
              Copy Key
            </button>
          </div>
        ),
        duration: 30000, // Show for 30 seconds
      });
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to generate key',
        variant: 'destructive',
      });
    }
  };

  const handleRevokeKey = async (keyId: string) => {
    if (!confirm('Are you sure you want to revoke this key? This will make it unusable immediately.')) {
      return;
    }

    try {
      await revokeKeyMutation(keyId);
      toast({
        title: 'Success',
        description: 'Key revoked successfully',
      });
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to revoke key',
        variant: 'destructive',
      });
    }
  };

  const handleToggleKeyStatus = async (keyId: string, isActive: boolean) => {
    if (!confirm(`Are you sure you want to ${isActive ? 'disable' : 'enable'} this key?`)) {
      return;
    }

    try {
      await toggleKeyStatusMutation({ keyId, isActive });
      toast({
        title: 'Success',
        description: `Key ${!isActive ? 'enabled' : 'disabled'} successfully`,
      });
    } catch (error) {
      toast({
        title: 'Error',
        description: `Failed to ${!isActive ? 'enable' : 'disable'} key`,
        variant: 'destructive',
      });
    }
  };

  const handleDeleteKey = async (keyId: string, key: ApiKey) => {
    // Check if user has permission to delete this key
    const canDelete = isAdmin || (key.user_id && !key.team_id); // Users can only delete their own personal keys
    if (!canDelete) {
      toast({
        title: 'Error',
        description: 'You do not have permission to delete this key',
        variant: 'destructive',
      });
      return;
    }

    if (!confirm('Are you sure you want to delete this key? This action cannot be undone.')) {
      return;
    }

    try {
      await deleteKeyMutation(keyId);
      toast({
        title: 'Success',
        description: 'Key deleted successfully',
      });
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to delete key',
        variant: 'destructive',
      });
    }
  };

  const getKeyStatus = (key: ApiKey) => {
    if (key.revoked_at) return 'revoked';
    if (key.expires_at && new Date(key.expires_at) < new Date()) return 'expired';
    if (!key.is_active) return 'inactive';
    return 'active';
  };

  if (isLoading) {
    return <LoadingState text="Loading API keys..." />;
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={isAdmin ? 'All API Keys' : 'My API Keys'}
        description="Manage API keys and monitor usage"
        actions={
          <CreateKeyDialog
            isAdmin={isAdmin}
            userTeams={userTeams}
            onCreateKey={handleGenerateKey}
          />
        }
      />

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Total Keys"
          value={keys.length.toString()}
          icon={Key}
        />
        <StatCard
          title="Active Keys"
          value={keys.filter(k => getKeyStatus(k) === 'active').length.toString()}
          description={`${keys.filter(k => getKeyStatus(k) === 'inactive').length} inactive`}
          icon={CheckCircle}
        />
        <StatCard
          title="Total Usage"
          value={keys.reduce((sum, k) => sum + k.usage_count, 0).toLocaleString()}
          description="requests"
          icon={Activity}
        />
        <StatCard
          title="Total Spend"
          value={`$${keys.reduce((sum, k) => sum + k.current_spend, 0).toFixed(2)}`}
          description="cumulative"
          icon={DollarSign}
        />
      </div>

      {/* Data Table */}
      <DataTable
        columns={createColumns(handleToggleKeyStatus, handleRevokeKey, handleDeleteKey)}
        data={keys}
      />
    </div>
  );
};

export default Keys;