import { Navigate, Route, Routes } from "react-router-dom";
import { useAuthStore } from "@/stores/auth";
import { Layout } from "@/components/Layout";
import { Login } from "@/pages/Login";
import { Clients } from "@/pages/Clients";
import { ApiTokens } from "@/pages/ApiTokens";
import { Messages } from "@/pages/Messages";
import { Settings } from "@/pages/Settings";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const password = useAuthStore((s) => s.password);
  if (!password) return <Navigate to="/login" replace />;
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
        <Route path="/tokens" element={<ApiTokens />} />
        <Route path="/messages" element={<Messages />} />
        <Route path="/settings" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
