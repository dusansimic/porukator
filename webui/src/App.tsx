import { Navigate, Route, Routes } from "react-router-dom";
import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { AdminService, Role } from "@/gen/porukator/v1/porukator_pb";
import { transport } from "@/lib/transport";
import { useAuthStore, isAdmin } from "@/stores/auth";
import { Layout } from "@/components/Layout";
import { Login } from "@/pages/Login";
import { Clients } from "@/pages/Clients";
import { ApiTokens } from "@/pages/ApiTokens";
import { Messages } from "@/pages/Messages";
import { Settings } from "@/pages/Settings";
import { Users } from "@/pages/Users";

const admin = createClient(AdminService, transport);

// Gate the app on a token and validate it once on mount (the session may have
// been revoked or the role changed server-side). On failure, clear + redirect.
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token);
  const setUser = useAuthStore((s) => s.setUser);
  const clear = useAuthStore((s) => s.clear);
  const qc = useQueryClient();

  useEffect(() => {
    if (!token) return;
    admin
      .getCurrentUser({})
      .then((res) => {
        if (res.user) {
          setUser({ id: res.user.id, username: res.user.username, role: res.user.role });
        }
      })
      .catch(() => {
        clear();
        qc.clear();
      });
  }, [token, setUser, clear, qc]);

  if (!token) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

// AdminRoute additionally requires the admin role.
function AdminRoute({ children }: { children: React.ReactNode }) {
  const user = useAuthStore((s) => s.user);
  if (!isAdmin(user)) return <Navigate to="/clients" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route path="/" element={<Navigate to="/clients" replace />} />
        <Route path="/clients" element={<Clients />} />
        <Route path="/messages" element={<Messages />} />
        <Route path="/tokens" element={<AdminRoute><ApiTokens /></AdminRoute>} />
        <Route path="/settings" element={<AdminRoute><Settings /></AdminRoute>} />
        <Route path="/users" element={<AdminRoute><Users /></AdminRoute>} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

// Re-export for convenience in pages that need ad-hoc admin calls.
export { Role };
