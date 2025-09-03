import React, { useState, useEffect } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { useToast } from '../components/ui/use-toast';
import { usePermissions } from '../contexts/PermissionContext';
import { DataTable } from '../components/keys/data-table';
import { createColumns, ApiKey } from '../components/keys/columns';
import { CreateKeyDialog } from '../components/keys/create-key-dialog';
import api from '../lib/api';

// Using ApiKey interface from columns.tsx

const Keys: React.FC = () => {
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [userTeams, setUserTeams] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const { toast } = useToast();
  const { hasRole, hasPermission } = usePermissions();
  
  // Check if user is admin
  const isAdmin = hasRole('admin') || hasPermission('admin.*');

  useEffect(() => {
    fetchKeys();
    if (!isAdmin) {
      fetchUserTeams();
    }
  }, [isAdmin]);

  const fetchKeys = async () => {
    try {
      const data = await api.keys.list(!isAdmin) as any;
      setKeys(data.keys || []);
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to fetch keys',
        variant: 'destructive',
      });
    } finally {
      setLoading(false);
    }
  };

  const fetchUserTeams = async () => {
    try {
      // For non-admin users, we need to fetch their teams
      const data = await api.userProfile.getTeams() as any;
      setUserTeams(data.teams || []);
    } catch (error) {
      console.error('Failed to fetch user teams:', error);
      // Not critical, so don't show error toast
    }
  };

  const generateKey = async (keyData: any) => {
    try {
      const data = await api.keys.generate(keyData, !isAdmin) as any;
      
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
      
      fetchKeys(); // Refresh the list
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to generate key',
        variant: 'destructive',
      });
    }
  };

  const revokeKey = async (keyId: string) => {
    if (!confirm('Are you sure you want to revoke this key? This will make it unusable immediately.')) {
      return;
    }

    try {
      await api.keys.revoke(keyId, { reason: 'Revoked by admin' });
      toast({
        title: 'Success',
        description: 'Key revoked successfully',
      });
      fetchKeys();
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to revoke key',
        variant: 'destructive',
      });
    }
  };

  const toggleKeyStatus = async (keyId: string, isActive: boolean) => {
    if (!confirm(`Are you sure you want to ${isActive ? 'disable' : 'enable'} this key?`)) {
      return;
    }

    try {
      await api.keys.update(keyId, { is_active: !isActive });
      toast({
        title: 'Success',
        description: `Key ${!isActive ? 'enabled' : 'disabled'} successfully`,
      });
      fetchKeys();
    } catch (error) {
      toast({
        title: 'Error',
        description: `Failed to ${!isActive ? 'enable' : 'disable'} key`,
        variant: 'destructive',
      });
    }
  };

  const deleteKey = async (keyId: string, key: ApiKey) => {
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
      await api.keys.delete(keyId, !isAdmin);
      toast({
        title: 'Success',
        description: 'Key deleted successfully',
      });
      setKeys(keys.filter(k => k.id !== keyId));
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

  if (loading) {
    return <div className="flex items-center justify-center h-96">Loading...</div>;
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex items-center gap-2">
          <h1 className="text-3xl font-bold">
            {isAdmin ? 'All API Keys' : 'My API Keys'}
          </h1>
          {isAdmin && <Badge variant="secondary">Admin View</Badge>}
        </div>
        <CreateKeyDialog 
          isAdmin={isAdmin}
          userTeams={userTeams}
          onCreateKey={generateKey}
        />
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Keys</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{keys.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Keys</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600 dark:text-green-400">
              {keys.filter(k => getKeyStatus(k) === 'active').length}
            </div>
            <div className="text-xs text-muted-foreground">
              {keys.filter(k => getKeyStatus(k) === 'inactive').length} inactive
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Usage</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {keys.reduce((sum, k) => sum + k.usage_count, 0).toLocaleString()}
            </div>
            <div className="text-xs text-muted-foreground">requests</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Spend</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              ${keys.reduce((sum, k) => sum + k.current_spend, 0).toFixed(2)}
            </div>
            <div className="text-xs text-muted-foreground">cumulative</div>
          </CardContent>
        </Card>
      </div>

      {/* Data Table */}
      <DataTable 
        columns={createColumns(toggleKeyStatus, revokeKey, deleteKey)}
        data={keys}
      />
    </div>
  );
};

export default Keys;