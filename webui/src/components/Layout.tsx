import { createClient } from "@connectrpc/connect";
import {
  KeyRound,
  LogOut,
  MessageSquare,
  SlidersHorizontal,
  Smartphone,
  Users as UsersIcon,
} from "lucide-react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { AdminService, Role } from "@/gen/porukator/v1/porukator_pb";
import { transport } from "@/lib/transport";
import { cn } from "@/lib/utils";
import { isAdmin, useAuthStore } from "@/stores/auth";

const admin = createClient(AdminService, transport);

// adminOnly entries are hidden from managers.
const nav = [
  { to: "/clients", label: "Devices", icon: Smartphone, adminOnly: false },
  { to: "/messages", label: "Messages", icon: MessageSquare, adminOnly: false },
  { to: "/tokens", label: "API Tokens", icon: KeyRound, adminOnly: false },
  { to: "/settings", label: "Settings", icon: SlidersHorizontal, adminOnly: true },
  { to: "/users", label: "Users", icon: UsersIcon, adminOnly: true },
];

export function Layout() {
  const user = useAuthStore((s) => s.user);
  const clear = useAuthStore((s) => s.clear);
  const navigate = useNavigate();
  const userIsAdmin = isAdmin(user);

  async function signOut() {
    // Best-effort server-side revoke; clear locally regardless.
    await admin.logout({}).catch(() => {});
    clear();
    navigate("/login", { replace: true });
  }

  return (
    <div className="min-h-screen flex">
      <aside className="w-60 border-r bg-card flex flex-col">
        <div className="p-5">
          <div className="text-lg font-semibold tracking-tight">Porukator</div>
          <div className="text-xs text-muted-foreground font-mono" title="build commit">
            {__GIT_SHA__}
          </div>
        </div>
        <nav className="flex-1 px-2 space-y-1">
          {nav
            .filter((n) => !n.adminOnly || userIsAdmin)
            .map(({ to, label, icon: Icon }) => (
              <NavLink
                key={to}
                to={to}
                className={({ isActive }) =>
                  cn(
                    "flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors",
                    isActive
                      ? "bg-secondary text-foreground"
                      : "text-muted-foreground hover:bg-secondary/60",
                  )
                }
              >
                <Icon className="h-4 w-4" />
                {label}
              </NavLink>
            ))}
        </nav>
        <div className="p-2 space-y-1">
          {user && (
            <div className="px-3 py-1 text-xs text-muted-foreground">
              {user.username}
              <span className="ml-1 opacity-70">
                ({user.role === Role.ADMIN ? "admin" : "manager"})
              </span>
            </div>
          )}
          <Button
            variant="ghost"
            className="w-full justify-start text-muted-foreground"
            onClick={signOut}
          >
            <LogOut className="h-4 w-4" /> Sign out
          </Button>
        </div>
      </aside>
      <main className="flex-1 p-8 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
