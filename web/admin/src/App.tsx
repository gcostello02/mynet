import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./hooks/useAuth";
import { AppShell } from "./layouts/AppShell";
import { DevicesPage } from "./pages/DevicesPage";
import { InvitationsPage } from "./pages/InvitationsPage";
import { LoginPage } from "./pages/LoginPage";
import { OverviewPage } from "./pages/OverviewPage";
import { SettingsPage } from "./pages/SettingsPage";

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { checked, authenticated } = useAuth();
  if (!checked) {
    return <p className="p-6 text-zinc-500">Checking session…</p>;
  }
  if (!authenticated) {
    return <Navigate to="/login" replace />;
  }
  return children;
}

export default function App() {
  return (
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
  );
}
