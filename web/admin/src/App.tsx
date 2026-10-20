import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./hooks/useAuth";
import { ToastProvider } from "./hooks/useToast";
import { AppShell } from "./layouts/AppShell";
import { DevicesPage } from "./pages/DevicesPage";
import { InvitationsPage } from "./pages/InvitationsPage";
import { LoginPage } from "./pages/LoginPage";
import { OverviewPage } from "./pages/OverviewPage";
import { SettingsPage } from "./pages/SettingsPage";
import { Skeleton } from "./components/ui/Skeleton";

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { checked, authenticated, sessionExpired } = useAuth();
  if (!checked) {
    return (
      <div className="flex min-h-screen items-center justify-center p-8">
        <div className="w-full max-w-xs space-y-3">
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-10 w-full" />
        </div>
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
    <AuthProvider>
      <ToastProvider>
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
      </ToastProvider>
    </AuthProvider>
  );
}
