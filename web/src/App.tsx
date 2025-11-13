import { lazy, Suspense, useEffect, useState } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import { useAuthStore } from './stores/authStore';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ToastProvider } from './contexts/ToastContext';
import Layout from './components/Layout';
import { decodeJWT } from './utils/jwt';
import Setup from './pages/Setup';

// Lazy load page components for code splitting
const Dashboard = lazy(() => import('./pages/Dashboard'));
const HostList = lazy(() => import('./pages/HostList'));
const HostDetail = lazy(() => import('./pages/HostDetail'));
const Containers = lazy(() => import('./pages/Containers'));
const Stacks = lazy(() => import('./pages/Stacks'));
const Settings = lazy(() => import('./pages/Settings'));

// Create a client
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

// Loading fallback component
const PageLoader = () => (
  <div className="flex items-center justify-center h-screen">
    <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
  </div>
);

interface ProtectedProps {
  readonly children: React.ReactElement;
}

function Protected({ children }: ProtectedProps) {
  const token = useAuthStore((s) => s.accessToken);
  const setAccessToken = useAuthStore((s) => s.setAccessToken);
  const setCsrfToken = useAuthStore((s) => s.setCsrfToken);
  const setAuth = useAuthStore((s) => s.setAuth);
  const [bootstrapping, setBootstrapping] = useState(!token);

  useEffect(() => {
    // Attempt silent refresh on first mount if no token but we might have refresh cookie
    if (!token) {
      fetch('/api/v1/auth/refresh', {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          // Attach CSRF from cookie if present
          'X-CSRF-Token': (() => {
            if (typeof document === 'undefined') return '';
            const regex = /(?:^|; )flotilla_csrf=([^;]+)/;
            const match = regex.exec(document.cookie);
            return match ? decodeURIComponent(match[1]) : '';
          })(),
        },
      })
        .then(async (res) => {
          if (res.ok) {
            const data = await res.json();
            const newCsrf = res.headers.get('X-CSRF-Token');
            if (data?.access_token) {
              // Decode JWT to restore user info
              const claims = decodeJWT(data.access_token);
              if (claims?.username && claims?.role && claims?.sub) {
                setAuth(data.access_token, newCsrf ?? '', {
                  id: claims.sub,
                  username: claims.username,
                  role: claims.role,
                });
              } else {
                setAccessToken(data.access_token);
                if (newCsrf) setCsrfToken(newCsrf);
              }
            }
          }
        })
        .finally(() => setBootstrapping(false));
    } else {
      // If we have a token but no user info, decode it to restore user info
      const user = useAuthStore.getState().user;
      if (!user && token) {
        const claims = decodeJWT(token);
        if (claims?.username && claims?.role && claims?.sub) {
          setAuth(token, useAuthStore.getState().csrfToken ?? '', {
            id: claims.sub,
            username: claims.username,
            role: claims.role,
          });
        }
      }
      setBootstrapping(false);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!token && bootstrapping) return <PageLoader />;
  if (!token) return <Navigate to="/login" replace />;
  return children;
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <Router>
          <Layout>
            <Suspense fallback={<PageLoader />}>
              <Routes>
                <Route path="/login" element={<Login />} />
                <Route path="/setup" element={<Setup />} />
                <Route path="/" element={<Navigate to="/dashboard" replace />} />
                <Route path="/dashboard" element={<Protected><Dashboard /></Protected>} />
                <Route path="/hosts" element={<Protected><HostList /></Protected>} />
                <Route path="/hosts/:hostId" element={<Protected><HostDetail /></Protected>} />
                <Route path="/containers" element={<Protected><Containers /></Protected>} />
                <Route path="/stacks" element={<Protected><Stacks /></Protected>} />
                <Route path="/settings/*" element={<Protected><Settings /></Protected>} />
              </Routes>
            </Suspense>
          </Layout>
        </Router>
      </ToastProvider>
    </QueryClientProvider>
  );
}

export default App;
