import React from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { ServerStoreProvider } from './store/useServerStore';
import { AppLayout } from './components/layout/AppLayout';
import { ErrorBoundary } from './components/layout/ErrorBoundary';
import { appRoutes } from './routes';

export const AppContent: React.FC = () => {
  const location = useLocation();

  return (
    <AppLayout>
      <ErrorBoundary key={location.pathname}>
        <Routes>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          {appRoutes.map((route) => (
            <Route key={route.id} path={route.path} element={route.element} />
          ))}
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </ErrorBoundary>
    </AppLayout>
  );
};

function App() {
  return (
    <BrowserRouter>
      <ServerStoreProvider>
        <AppContent />
      </ServerStoreProvider>
    </BrowserRouter>
  );
}

export default App;
