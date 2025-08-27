import React, { useState, useEffect } from 'react';
import { Plus, Key, Copy, Trash2, RotateCw, Eye, EyeOff, Users, User } from 'lucide-react';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Badge } from '../components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select';
import { useToast } from '../components/ui/use-toast';
import { usePermissions } from '../contexts/PermissionContext';
import { CanAccess } from '../components/CanAccess';
import api from '../lib/api';

interface VirtualKey {
  id: string;
  key?: string; // Only present when creating new keys
  key_prefix?: string; // May not be present in all responses
  name: string;
  user_id?: string;
  team_id?: string;
  is_active: boolean;
  expires_at?: string;
  max_budget?: number;
  current_spend: number;
  tpm?: number;
  rpm?: number;
  allowed_models?: string[];
  blocked_models?: string[];
  usage_count: number;
  total_tokens: number;
  last_used_at?: string;
  created_at: string;
  revoked_at?: string;
  metadata?: any;
}

const Keys: React.FC = () => {
  const [keys, setKeys] = useState<VirtualKey[]>([]);
  const [userTeams, setUserTeams] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showKeyValue, setShowKeyValue] = useState<{[key: string]: boolean}>({});
  const [filter, setFilter] = useState<'all' | 'active' | 'expired' | 'revoked'>('all');
  const [searchTerm, setSearchTerm] = useState('');
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
      
      // Show the new key in a modal or alert
      toast({
        title: 'Key Generated Successfully',
        description: (
          <div className="mt-2">
            <p className="mb-2">Save this key - it won't be shown again!</p>
            <code className="block p-2 bg-gray-100 rounded text-xs break-all">
              {data.key || data.key_value}
            </code>
            <Button
              size="sm"
              variant="outline"
              className="mt-2"
              onClick={() => navigator.clipboard.writeText(data.key || data.key_value)}
            >
              <Copy className="h-3 w-3 mr-1" />
              Copy
            </Button>
          </div>
        ),
        duration: 30000, // Show for 30 seconds
      });
      
      fetchKeys(); // Refresh the list
      setShowCreateModal(false);
    } catch (error) {
      toast({
        title: 'Error',
        description: 'Failed to generate key',
        variant: 'destructive',
      });
    }
  };

  const revokeKey = async (keyId: string) => {
    // Only admins can revoke keys
    if (!isAdmin) {
      toast({
        title: 'Error',
        description: 'Only administrators can revoke keys',
        variant: 'destructive',
      });
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

  const deleteKey = async (keyId: string) => {
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

  const copyKey = (key: string) => {
    navigator.clipboard.writeText(key);
    toast({
      title: 'Copied',
      description: 'Key copied to clipboard',
    });
  };

  const toggleKeyVisibility = (keyId: string) => {
    setShowKeyValue(prev => ({
      ...prev,
      [keyId]: !prev[keyId]
    }));
  };

  const getKeyStatus = (key: VirtualKey) => {
    if (key.revoked_at) return 'revoked';
    if (key.expires_at && new Date(key.expires_at) < new Date()) return 'expired';
    if (!key.is_active) return 'inactive';
    return 'active';
  };

  const getBudgetPercentage = (key: VirtualKey) => {
    if (!key.max_budget || key.max_budget === 0) return 0;
    return (key.current_spend / key.max_budget) * 100;
  };

  const filteredKeys = keys.filter(key => {
    const status = getKeyStatus(key);
    if (filter !== 'all' && status !== filter) return false;
    if (searchTerm && !key.name.toLowerCase().includes(searchTerm.toLowerCase())) return false;
    return true;
  });

  if (loading) {
    return <div className="flex items-center justify-center h-96">Loading...</div>;
  }

  return (
    <div className="p-6">
      <div className="flex justify-between items-center mb-6">
        <div className="flex items-center gap-2">
          <h1 className="text-3xl font-bold">
            {isAdmin ? 'All API Keys' : 'My API Keys'}
          </h1>
          {isAdmin && <Badge variant="secondary">Admin View</Badge>}
        </div>
        <Button onClick={() => setShowCreateModal(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {isAdmin ? 'Generate Key' : 'Create API Key'}
        </Button>
      </div>

      {/* Filters */}
      <div className="flex gap-4 mb-6">
        <Input
          placeholder="Search keys..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="max-w-sm"
        />
        <Select value={filter} onValueChange={(value: any) => setFilter(value)}>
          <SelectTrigger className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Keys</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="expired">Expired</SelectItem>
            <SelectItem value="revoked">Revoked</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Keys</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{keys.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Active Keys</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {keys.filter(k => getKeyStatus(k) === 'active').length}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Usage</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {keys.reduce((sum, k) => sum + k.usage_count, 0).toLocaleString()}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Spend</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              ${keys.reduce((sum, k) => sum + k.current_spend, 0).toFixed(2)}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Keys List */}
      <div className="space-y-4">
        {filteredKeys.map((key) => {
          const status = getKeyStatus(key);
          return (
            <Card key={key.id}>
              <CardContent className="p-6">
                <div className="flex items-start justify-between">
                  <div className="space-y-3 flex-1">
                    <div className="flex items-center gap-2">
                      <Key className="h-5 w-5 text-gray-500" />
                      <h3 className="font-semibold">{key.name}</h3>
                      <Badge
                        variant={
                          status === 'active' ? 'default' :
                          status === 'expired' ? 'secondary' :
                          status === 'revoked' ? 'destructive' :
                          'outline'
                        }
                      >
                        {status}
                      </Badge>
                    </div>
                    
                    <div className="flex items-center gap-2">
                      <code className="text-sm bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded">
                        {showKeyValue[key.id] && key.key ? key.key : `****${key.key_prefix || 'xxxx'}`}
                      </code>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => toggleKeyVisibility(key.id)}
                      >
                        {showKeyValue[key.id] ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={!key.key}
                        onClick={() => key.key && copyKey(key.key)}
                      >
                        <Copy className="h-4 w-4" />
                      </Button>
                    </div>

                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                      <div>
                        <span className="text-gray-500">Usage:</span>
                        <span className="ml-2 font-medium">{key.usage_count} requests</span>
                      </div>
                      <div>
                        <span className="text-gray-500">Tokens:</span>
                        <span className="ml-2 font-medium">{key.total_tokens.toLocaleString()}</span>
                      </div>
                      {key.max_budget && (
                        <div>
                          <span className="text-gray-500">Budget:</span>
                          <span className="ml-2 font-medium">
                            ${key.current_spend.toFixed(2)} / ${key.max_budget.toFixed(2)}
                          </span>
                        </div>
                      )}
                      {key.expires_at && (
                        <div>
                          <span className="text-gray-500">Expires:</span>
                          <span className="ml-2 font-medium">
                            {new Date(key.expires_at).toLocaleDateString()}
                          </span>
                        </div>
                      )}
                    </div>

                    {key.max_budget && (
                      <div>
                        <div className="flex justify-between text-sm mb-1">
                          <span>Budget Usage</span>
                          <span>{getBudgetPercentage(key).toFixed(1)}%</span>
                        </div>
                        <div className="w-full bg-gray-200 rounded-full h-2">
                          <div
                            className={`h-2 rounded-full ${
                              getBudgetPercentage(key) > 80 ? 'bg-red-500' :
                              getBudgetPercentage(key) > 50 ? 'bg-yellow-500' :
                              'bg-green-500'
                            }`}
                            style={{ width: `${Math.min(getBudgetPercentage(key), 100)}%` }}
                          />
                        </div>
                      </div>
                    )}

                    <div className="flex gap-4 text-xs text-gray-500">
                      <span>Created: {new Date(key.created_at).toLocaleDateString()}</span>
                      {key.last_used_at && (
                        <span>Last used: {new Date(key.last_used_at).toLocaleDateString()}</span>
                      )}
                      {key.team_id && <span>Team key</span>}
                      {key.user_id && <span>User key</span>}
                    </div>
                  </div>

                  <div className="flex gap-2 ml-4">
                    <CanAccess role="admin" permissions={['admin.keys.revoke']}>
                      {status === 'active' && (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => revokeKey(key.id)}
                        >
                          <RotateCw className="h-4 w-4 mr-1" />
                          Revoke
                        </Button>
                      )}
                    </CanAccess>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => deleteKey(key.id)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>

      {/* Create Key Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-[500px] max-h-[90vh] overflow-y-auto">
            <CardHeader>
              <CardTitle>Generate New API Key</CardTitle>
              <CardDescription>Configure the key settings and permissions</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label>Key Name</Label>
                <Input id="key-name" placeholder="Enter a descriptive name" />
              </div>

{isAdmin ? (
                <div>
                  <Label>Ownership</Label>
                  <Select defaultValue="user" onValueChange={(value) => {
                    const ownershipSelect = document.getElementById('key-ownership') as HTMLSelectElement;
                    if (ownershipSelect) ownershipSelect.value = value;
                  }}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="user">User Key</SelectItem>
                      <SelectItem value="team">Team Key</SelectItem>
                      <SelectItem value="system">System Key</SelectItem>
                    </SelectContent>
                  </Select>
                  <select id="key-ownership" className="hidden" defaultValue="user" />
                </div>
              ) : userTeams.length > 0 ? (
                <div>
                  <Label>Key Type</Label>
                  <Select defaultValue="personal" onValueChange={(value) => {
                    const keyTypeSelect = document.getElementById('key-type') as HTMLSelectElement;
                    if (keyTypeSelect) keyTypeSelect.value = value;
                  }}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="personal">
                        <div className="flex items-center gap-2">
                          <User className="h-4 w-4" />
                          Personal Key
                        </div>
                      </SelectItem>
                      {userTeams.map((team) => (
                        <SelectItem key={team.id} value={`team:${team.id}`}>
                          <div className="flex items-center gap-2">
                            <Users className="h-4 w-4" />
                            {team.name} Team
                          </div>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <select id="key-type" className="hidden" defaultValue="personal" />
                </div>
              ) : null}

              <div>
                <Label>Expiration</Label>
                <Select defaultValue="never">
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="never">Never</SelectItem>
                    <SelectItem value="7">7 days</SelectItem>
                    <SelectItem value="30">30 days</SelectItem>
                    <SelectItem value="90">90 days</SelectItem>
                    <SelectItem value="365">1 year</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div>
                <Label>Budget Limit ($)</Label>
                <Input id="key-budget" type="number" placeholder="Leave empty for unlimited" />
              </div>

              <div>
                <Label>Budget Period</Label>
                <Select defaultValue="monthly">
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="daily">Daily</SelectItem>
                    <SelectItem value="weekly">Weekly</SelectItem>
                    <SelectItem value="monthly">Monthly</SelectItem>
                    <SelectItem value="yearly">Yearly</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div>
                <Label>Rate Limits</Label>
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <Label className="text-xs">TPM (Tokens/min)</Label>
                    <Input id="key-tpm" type="number" placeholder="100000" />
                  </div>
                  <div>
                    <Label className="text-xs">RPM (Requests/min)</Label>
                    <Input id="key-rpm" type="number" placeholder="60" />
                  </div>
                </div>
              </div>

              <div className="flex justify-end space-x-2">
                <Button variant="outline" onClick={() => setShowCreateModal(false)}>
                  Cancel
                </Button>
                <Button onClick={() => {
                  const name = (document.getElementById('key-name') as HTMLInputElement)?.value;
                  const budget = (document.getElementById('key-budget') as HTMLInputElement)?.value;
                  const tpm = (document.getElementById('key-tpm') as HTMLInputElement)?.value;
                  const rpm = (document.getElementById('key-rpm') as HTMLInputElement)?.value;
                  const ownership = (document.getElementById('key-ownership') as HTMLSelectElement)?.value;
                  const keyType = (document.getElementById('key-type') as HTMLSelectElement)?.value;
                  
                  let keyData: any = {
                    name: name || 'New API Key',
                    max_budget: budget ? parseFloat(budget) : undefined,
                    budget_duration: 'monthly',
                    tpm: tpm ? parseInt(tpm) : undefined,
                    rpm: rpm ? parseInt(rpm) : undefined,
                  };

                  // Handle ownership/key type for different user roles
                  if (isAdmin && ownership) {
                    keyData.type = ownership === 'system' ? 'master' : 'api';
                  } else if (!isAdmin && keyType) {
                    if (keyType.startsWith('team:')) {
                      keyData.team_id = keyType.replace('team:', '');
                    }
                    // Personal keys are the default (no team_id)
                  }
                  
                  generateKey(keyData);
                }}>
                  Generate Key
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
};

export default Keys;