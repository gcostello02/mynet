import type { ReactNode } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { Logo } from "./components/brand/Logo";
import { Skeleton } from "./components/ui/Skeleton";
import { AuthProvider, useAuth } from "./hooks/useAuth";
import { ToastProvider } from "./hooks/useToast";
import { AppShell } from "./layouts/AppShell";
import { DevicesPage } from "./pages/DevicesPage";
import { InvitationsPage } from "./pages/InvitationsPage";
import { LoginPage } from "./pages/LoginPage";
import { OverviewPage } from "./pages/OverviewPage";
import { SettingsPage } from "./pages/SettingsPage";

function RequireAuth({ children }: { children: ReactNode }) {
  const { checked, authenticated, sessionExpired } = useAuth();
  if (!checked) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center gap-6" role="status" aria-label="Checking session">
        <Logo />
        <Skeleton className="h-1 w-24" />
      </div>
    );
  }
  if (!authenticated) {
    return <Navigate to={sessionExpired ? "/login?expired=1" : "/login"} replace />;
  }
  return children;
}

export default function App() {
  return (
    <ToastProvider>
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route
              element={
                <RequireAuth>
                  <AppShell />
                </RequireAuth>
              }
            >
              <Route index element={<OverviewPage />} />
              <Route path="devices" element={<DevicesPage />} />
              <Route path="invitations" element={<InvitationsPage />} />
              <Route path="settings" element={<SettingsPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </ToastProvider>
  );
}
