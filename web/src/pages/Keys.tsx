import React from 'react';
import { useToast } from '../components/ui/use-toast';
import { usePermissions } from '../contexts/PermissionContext';
import { DataTable } from '../components/keys/data-table';
import { createColumns, ApiKey, getKeyStatus } from '../components/keys/columns';
import { CreateKeyDialog } from '../components/keys/create-key-dialog';
import { LoadingState } from '../components/common/LoadingState';
import { Icon } from '@iconify/react';
import { icons } from '@/lib/icons';
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

  if (isLoading) {
    return <LoadingState text="Loading API keys..." />;
  }

  const activeKeys = keys.filter(k => getKeyStatus(k) === 'active').length;
  const inactiveKeys = keys.filter(k => getKeyStatus(k) === 'inactive').length;
  const revokedKeys = keys.filter(k => getKeyStatus(k) === 'revoked').length;
  const totalRequests = keys.reduce((sum, k) => sum + k.usage_count, 0);
  const totalSpend = keys.reduce((sum, k) => sum + k.current_spend, 0);

  // Calculate bar widths for the stacked bar
  const total = keys.length || 1;
  const activePercent = (activeKeys / total) * 100;
  const inactivePercent = (inactiveKeys / total) * 100;
  const revokedPercent = (revokedKeys / total) * 100;

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-bold tracking-tight">
            {isAdmin ? 'API Keys' : 'My API Keys'}
          </h1>
          <p className="text-[13px] text-muted-foreground">
            Manage API access to your gateway
          </p>
        </div>
        <CreateKeyDialog
          isAdmin={isAdmin}
          userTeams={userTeams}
          onCreateKey={handleGenerateKey}
        />
      </div>

      {/* Quick Stats Row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        {/* Total Keys */}
        <div className="rounded-lg border bg-card p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Total Keys</span>
            <Icon icon={icons.keys} className="h-4 w-4 text-muted-foreground" />
          </div>
          <div className="text-2xl font-bold font-mono">{keys.length}</div>
          {keys.length > 0 && (
            <div className="mt-2 space-y-1">
              <div className="flex h-1.5 w-full rounded-full overflow-hidden bg-muted">
                <div className="bg-emerald-500 transition-all" style={{ width: `${activePercent}%` }} />
                <div className="bg-zinc-400 transition-all" style={{ width: `${inactivePercent}%` }} />
                <div className="bg-red-500 transition-all" style={{ width: `${revokedPercent}%` }} />
              </div>
              <div className="flex gap-3 text-[10px] text-muted-foreground">
                <span>{activeKeys} active</span>
                <span>{inactiveKeys} inactive</span>
                {revokedKeys > 0 && <span>{revokedKeys} revoked</span>}
              </div>
            </div>
          )}
        </div>

        {/* Active Keys */}
        <div className="rounded-lg border bg-card p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Active Keys</span>
            <span className="inline-block h-2.5 w-2.5 rounded-full bg-emerald-500" />
          </div>
          <div className="text-2xl font-bold font-mono">{activeKeys}</div>
          <p className="text-[11px] text-muted-foreground mt-1">
            {keys.length > 0 ? `${Math.round((activeKeys / keys.length) * 100)}% of total` : 'No keys yet'}
          </p>
        </div>

        {/* Total Requests */}
        <div className="rounded-lg border bg-card p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Total Requests</span>
            <Icon icon={icons.trendingUp} className="h-4 w-4 text-muted-foreground" />
          </div>
          <div className="text-2xl font-bold font-mono tabular-nums">{totalRequests.toLocaleString()}</div>
          <p className="text-[11px] text-muted-foreground mt-1">across all keys</p>
        </div>

        {/* Total Spend */}
        <div className="rounded-lg border bg-card p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">Total Spend</span>
            <Icon icon={icons.budget} className="h-4 w-4 text-muted-foreground" />
          </div>
          <div className="text-2xl font-bold font-mono tabular-nums">
            <span className="text-muted-foreground text-lg">$</span>{totalSpend.toFixed(2)}
          </div>
          <p className="text-[11px] text-muted-foreground mt-1">cumulative</p>
        </div>
      </div>

      {/* Key Table */}
      <DataTable
        columns={createColumns(handleToggleKeyStatus, handleRevokeKey, handleDeleteKey)}
        data={keys}
        teams={userTeams}
      />
    </div>
  );
};

export default Keys;
