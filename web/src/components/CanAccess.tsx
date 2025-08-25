import React from 'react';
import { usePermissions } from '../contexts/PermissionContext';

interface CanAccessProps {
  permission?: string;
  permissions?: string[];
  role?: string;
  roles?: string[];
  requireAll?: boolean;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

/**
 * CanAccess component provides role-based access control for UI elements
 * 
 * Usage:
 * <CanAccess permission="admin.users.read">
 *   <UserManagement />
 * </CanAccess>
 * 
 * <CanAccess permissions={['admin.keys.read', 'admin.keys.write']} requireAll={true}>
 *   <KeyManagement />
 * </CanAccess>
 */
export const CanAccess: React.FC<CanAccessProps> = ({
  permission,
  permissions = [],
  role,
  roles = [],
  requireAll = false,
  children,
  fallback = null
}) => {
  const { hasRole, hasAnyPermission, hasAllPermissions } = usePermissions();

  // Build permissions array
  const allPermissions = [...permissions];
  if (permission) {
    allPermissions.push(permission);
  }

  // Build roles array
  const allRoles = [...roles];
  if (role) {
    allRoles.push(role);
  }

  // Check permissions
  let hasRequiredPermissions = true;
  if (allPermissions.length > 0) {
    if (requireAll) {
      hasRequiredPermissions = hasAllPermissions(allPermissions);
    } else {
      hasRequiredPermissions = hasAnyPermission(allPermissions);
    }
  }

  // Check roles
  let hasRequiredRoles = true;
  if (allRoles.length > 0) {
    if (requireAll) {
      hasRequiredRoles = allRoles.every(r => hasRole(r));
    } else {
      hasRequiredRoles = allRoles.some(r => hasRole(r));
    }
  }

  // Show children only if user has required permissions and roles
  if (hasRequiredPermissions && hasRequiredRoles) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
};

// Convenience components for common access patterns
export const AdminOnly: React.FC<{ children: React.ReactNode; fallback?: React.ReactNode }> = ({ 
  children, 
  fallback 
}) => (
  <CanAccess role="admin" fallback={fallback}>
    {children}
  </CanAccess>
);

export const UserManagementAccess: React.FC<{ children: React.ReactNode; fallback?: React.ReactNode }> = ({ 
  children, 
  fallback 
}) => (
  <CanAccess permissions={['admin.users.read', 'admin.users.write']} fallback={fallback}>
    {children}
  </CanAccess>
);

export const KeyManagementAccess: React.FC<{ children: React.ReactNode; fallback?: React.ReactNode }> = ({ 
  children, 
  fallback 
}) => (
  <CanAccess permissions={['admin.keys.read', 'admin.keys.write']} fallback={fallback}>
    {children}
  </CanAccess>
);

export const TeamManagementAccess: React.FC<{ children: React.ReactNode; fallback?: React.ReactNode }> = ({ 
  children, 
  fallback 
}) => (
  <CanAccess permissions={['admin.teams.read', 'admin.teams.write']} fallback={fallback}>
    {children}
  </CanAccess>
);

export default CanAccess;