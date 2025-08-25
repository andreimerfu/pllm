import React, { createContext, useContext, useState, useEffect } from 'react';
import api from '../lib/api';

interface Permission {
  id: string;
  name: string;
  description: string;
}

interface UserPermissions {
  user_id?: number;
  permissions: string[];  // Backend returns array of permission strings
  role: string;           // Backend returns single role string
  groups: string[];
  auth_type?: string;
}

interface PermissionContextType {
  permissions: Permission[];
  roles: string[];
  groups: string[];
  isLoading: boolean;
  error: string | null;
  hasPermission: (permission: string) => boolean;
  hasRole: (role: string) => boolean;
  hasGroup: (group: string) => boolean;
  hasAnyPermission: (permissions: string[]) => boolean;
  hasAllPermissions: (permissions: string[]) => boolean;
  refreshPermissions: () => Promise<void>;
}

const PermissionContext = createContext<PermissionContextType | null>(null);

interface PermissionProviderProps {
  children: React.ReactNode;
}

export const PermissionProvider: React.FC<PermissionProviderProps> = ({ children }) => {
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [roles, setRoles] = useState<string[]>([]);
  const [groups, setGroups] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadPermissions = async () => {
    try {
      setIsLoading(true);
      setError(null);

      // Get current user's permissions
      // Note: api.axios already extracts response.data via interceptor
      const data = await api.axios.get('/api/admin/auth/permissions') as UserPermissions;
      
      console.log('Permissions API data:', data);
      
      if (!data) {
        throw new Error('No data received from permissions API');
      }
      
      // Convert permission strings to Permission objects
      const permissionObjects: Permission[] = (data.permissions || []).map((perm: string) => ({
        id: perm,
        name: perm,
        description: `Permission: ${perm}`
      }));
      
      setPermissions(permissionObjects);
      setRoles(data.role ? [data.role] : []); // Convert single role to array
      setGroups(data.groups || []);
      
      console.log('Processed permissions:', permissionObjects);
      console.log('Processed roles:', data.role ? [data.role] : []);
      console.log('Processed groups:', data.groups || []);
    } catch (err: any) {
      console.error('Failed to load permissions:', err);
      setError(err.message || 'Failed to load permissions');
      
      // Set empty arrays on error to prevent access
      setPermissions([]);
      setRoles([]);
      setGroups([]);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadPermissions();
  }, []);

  const hasPermission = (permission: string): boolean => {
    if (!permission) return false;
    
    return permissions.some(p => {
      // Exact match
      if (p.name === permission) return true;
      
      // Wildcard match: admin.* matches admin.users.read, admin.users.write, etc.
      if (p.name.endsWith('.*')) {
        const prefix = p.name.slice(0, -1); // Remove the *
        return permission.startsWith(prefix);
      }
      
      return false;
    });
  };

  const hasRole = (role: string): boolean => {
    if (!role) return false;
    return roles.includes(role);
  };

  const hasGroup = (group: string): boolean => {
    if (!group) return false;
    return groups.includes(group);
  };

  const hasAnyPermission = (permissionList: string[]): boolean => {
    if (!permissionList || permissionList.length === 0) return true;
    return permissionList.some(p => hasPermission(p));
  };

  const hasAllPermissions = (permissionList: string[]): boolean => {
    if (!permissionList || permissionList.length === 0) return true;
    return permissionList.every(p => hasPermission(p));
  };

  const refreshPermissions = async () => {
    await loadPermissions();
  };

  const value: PermissionContextType = {
    permissions,
    roles,
    groups,
    isLoading,
    error,
    hasPermission,
    hasRole,
    hasGroup,
    hasAnyPermission,
    hasAllPermissions,
    refreshPermissions,
  };

  return (
    <PermissionContext.Provider value={value}>
      {children}
    </PermissionContext.Provider>
  );
};

export const usePermissions = (): PermissionContextType => {
  const context = useContext(PermissionContext);
  if (!context) {
    throw new Error('usePermissions must be used within a PermissionProvider');
  }
  return context;
};

// Hook for checking specific permissions with loading state
export const useHasPermission = (permission: string) => {
  const { hasPermission, isLoading } = usePermissions();
  return {
    hasPermission: hasPermission(permission),
    isLoading,
  };
};

// Hook for checking multiple permissions
export const useHasAnyPermission = (permissions: string[]) => {
  const { hasAnyPermission, isLoading } = usePermissions();
  return {
    hasPermission: hasAnyPermission(permissions),
    isLoading,
  };
};

// Hook for checking if user has all required permissions
export const useHasAllPermissions = (permissions: string[]) => {
  const { hasAllPermissions, isLoading } = usePermissions();
  return {
    hasPermission: hasAllPermissions(permissions),
    isLoading,
  };
};

// Hook for role checking
export const useHasRole = (role: string) => {
  const { hasRole, isLoading } = usePermissions();
  return {
    hasRole: hasRole(role),
    isLoading,
  };
};

export default PermissionContext;