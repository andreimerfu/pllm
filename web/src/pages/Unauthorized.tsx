import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card';
import { ShieldX, Home, ArrowLeft } from 'lucide-react';
import { useAuth } from '../contexts/OIDCAuthContext';

interface UnauthorizedProps {
  message?: string;
  showBackButton?: boolean;
  showHomeButton?: boolean;
}

const Unauthorized: React.FC<UnauthorizedProps> = ({ 
  message = "You don't have permission to access this resource.",
  showBackButton = true,
  showHomeButton = true 
}) => {
  const navigate = useNavigate();
  const { isAuthenticated } = useAuth();

  const handleGoBack = () => {
    if (!isAuthenticated) {
      navigate('/login');
    } else {
      window.history.length > 1 ? navigate(-1) : navigate('/dashboard');
    }
  };

  const handleGoHome = () => {
    if (!isAuthenticated) {
      navigate('/login');
    } else {
      navigate('/dashboard');
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full">
        <Card>
          <CardHeader className="text-center">
            <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-red-100">
              <ShieldX className="h-8 w-8 text-red-600" />
            </div>
            <CardTitle className="mt-4 text-2xl font-bold text-gray-900">
              Access Denied
            </CardTitle>
            <CardDescription className="mt-2 text-gray-600">
              {message}
            </CardDescription>
          </CardHeader>
          
          <CardContent className="space-y-4">
            <div className="text-sm text-gray-500 text-center">
              <p>
                If you believe this is an error, please contact your system administrator
                or request the necessary permissions.
              </p>
            </div>
            
            <div className="flex flex-col gap-3">
              {showBackButton && (
                <Button 
                  variant="outline" 
                  onClick={handleGoBack}
                  className="w-full"
                >
                  <ArrowLeft className="mr-2 h-4 w-4" />
                  {!isAuthenticated ? 'Go to Login' : 'Go Back'}
                </Button>
              )}
              
              {showHomeButton && (
                <Button 
                  onClick={handleGoHome}
                  className="w-full"
                >
                  <Home className="mr-2 h-4 w-4" />
                  {!isAuthenticated ? 'Go to Login' : 'Go to Dashboard'}
                </Button>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
};

export default Unauthorized;