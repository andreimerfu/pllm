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
import { PageShell, StatTile } from '@/components/common/page-shell';

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

  return (
    <PageShell
      title={isAdmin ? "API Keys" : "My keys"}
      description="Bearer credentials that route traffic through your policies, budgets, and use-case scopes."
      actions={
        <CreateKeyDialog
          isAdmin={isAdmin}
          userTeams={userTeams}
          onCreateKey={handleGenerateKey}
        />
      }
    >
      {/* KPI cards */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatTile
          label="Total keys"
          value={keys.length}
          icon={<Icon icon={icons.keys} />}
          iconColor="prompt"
          sub={
            keys.length > 0
              ? `${activeKeys} active · ${inactiveKeys} inactive${revokedKeys > 0 ? ` · ${revokedKeys} revoked` : ""}`
              : "No keys yet"
          }
        />
        <StatTile
          label="Active keys"
          value={activeKeys}
          tone={activeKeys > 0 ? "good" : "default"}
          icon={<Icon icon={icons.guardrails} />}
          iconColor="good"
          sub={keys.length > 0 ? `${Math.round((activeKeys / keys.length) * 100)}% of total` : undefined}
        />
        <StatTile
          label="Total requests"
          value={totalRequests.toLocaleString()}
          icon={<Icon icon={icons.trendingUp} />}
          iconColor="primary"
          sub="Across all keys"
        />
        <StatTile
          label="Total spend"
          value={`$${totalSpend.toFixed(2)}`}
          icon={<Icon icon={icons.budget} />}
          iconColor="agent"
          sub="Cumulative"
        />
      </div>

      {/* Key Table */}
      <DataTable
        columns={createColumns(handleToggleKeyStatus, handleRevokeKey, handleDeleteKey)}
        data={keys}
        teams={userTeams}
      />
    </PageShell>
  );
};

export default Keys;
