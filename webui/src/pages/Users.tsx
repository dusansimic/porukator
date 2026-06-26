import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Ban, CircleCheck, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { AdminService, Role, type Session, type User } from "@/gen/porukator/v1/porukator_pb";
import { useAuthStore } from "@/stores/auth";

function ts(t?: { seconds: bigint }) {
  return t ? new Date(Number(t.seconds) * 1000).toLocaleString() : "—";
}

export function Users() {
  const qc = useQueryClient();
  const me = useAuthStore((s) => s.user);
  const { data: users } = useQuery(AdminService.method.listUsers, {});
  const { data: sessions } = useQuery(
    AdminService.method.listSessions,
    {},
    { refetchInterval: 5000 },
  );

  const create = useMutation(AdminService.method.createUser);
  const setRole = useMutation(AdminService.method.setUserRole);
  const setDisabled = useMutation(AdminService.method.setUserDisabled);
  const deleteUser = useMutation(AdminService.method.deleteUser);
  const revokeSession = useMutation(AdminService.method.revokeSession);

  const [newUsername, setNewUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newRole, setNewRole] = useState<Role>(Role.MANAGER);
  const [open, setOpen] = useState(false);
  const [err, setErr] = useState("");

  const invalidate = () => qc.invalidateQueries();

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setErr("");
    try {
      await create.mutateAsync({ username: newUsername, password: newPassword, role: newRole });
      setNewUsername("");
      setNewPassword("");
      setNewRole(Role.MANAGER);
      setOpen(false);
      invalidate();
    } catch {
      setErr("Could not create user (username may be taken).");
    }
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Users</h1>
          <p className="text-muted-foreground text-sm">Web-UI accounts and active sessions.</p>
        </div>
        <Dialog
          open={open}
          onOpenChange={(o) => {
            setOpen(o);
            if (!o) setErr("");
          }}
        >
          <DialogTrigger asChild>
            <Button>
              <Plus className="h-4 w-4" /> New user
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>New user</DialogTitle>
              <DialogDescription>Create a web-UI account.</DialogDescription>
            </DialogHeader>
            <form onSubmit={onCreate} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="u">Username</Label>
                <Input
                  id="u"
                  value={newUsername}
                  autoFocus
                  onChange={(e) => setNewUsername(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="p">Password</Label>
                <Input
                  id="p"
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="r">Role</Label>
                <select
                  id="r"
                  className="h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm"
                  value={newRole}
                  onChange={(e) => setNewRole(Number(e.target.value) as Role)}
                >
                  <option value={Role.MANAGER}>manager</option>
                  <option value={Role.ADMIN}>admin</option>
                </select>
              </div>
              {err && <p className="text-sm text-destructive">{err}</p>}
              <Button type="submit" disabled={!newUsername || !newPassword || create.isPending}>
                Create
              </Button>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Username</TableHead>
            <TableHead>Role</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(users?.users ?? []).map((u: User) => {
            const isSelf = u.id === me?.id;
            return (
              <TableRow key={u.id}>
                <TableCell className="font-medium">
                  {u.username}
                  {isSelf && <span className="ml-2 text-xs text-muted-foreground">(you)</span>}
                </TableCell>
                <TableCell>
                  <select
                    className="h-8 rounded-md border border-input bg-transparent px-2 text-sm"
                    value={u.role}
                    onChange={async (e) => {
                      await setRole.mutateAsync({ id: u.id, role: Number(e.target.value) as Role });
                      invalidate();
                    }}
                  >
                    <option value={Role.MANAGER}>manager</option>
                    <option value={Role.ADMIN}>admin</option>
                  </select>
                </TableCell>
                <TableCell>
                  {u.disabled ? (
                    <Badge variant="destructive">disabled</Badge>
                  ) : (
                    <Badge variant="success">active</Badge>
                  )}
                </TableCell>
                <TableCell className="text-muted-foreground">{ts(u.createdAt)}</TableCell>
                <TableCell className="text-right space-x-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    title={u.disabled ? "Enable" : "Disable"}
                    onClick={async () => {
                      const verb = u.disabled ? "Enable" : "Disable";
                      if (
                        confirm(
                          `${verb} ${u.username}?${!u.disabled ? " This revokes all their sessions." : ""}`,
                        )
                      ) {
                        await setDisabled.mutateAsync({ id: u.id, disabled: !u.disabled });
                        invalidate();
                      }
                    }}
                  >
                    {u.disabled ? <CircleCheck className="h-4 w-4" /> : <Ban className="h-4 w-4" />}
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    title="Delete"
                    onClick={async () => {
                      if (confirm(`Delete ${u.username}? This cannot be undone.`)) {
                        await deleteUser.mutateAsync({ id: u.id });
                        invalidate();
                      }
                    }}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>

      <div>
        <h2 className="text-lg font-semibold tracking-tight mb-2">Active sessions</h2>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>User</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Last used</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(sessions?.sessions ?? []).map((s: Session) => (
              <TableRow key={s.id}>
                <TableCell className="font-medium">
                  {s.username}
                  {s.current && (
                    <span className="ml-2 text-xs text-muted-foreground">(this session)</span>
                  )}
                </TableCell>
                <TableCell className="text-muted-foreground">{ts(s.createdAt)}</TableCell>
                <TableCell className="text-muted-foreground">{ts(s.lastUsedAt)}</TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="ghost"
                    size="icon"
                    title="Revoke session"
                    onClick={async () => {
                      if (
                        confirm(
                          s.current
                            ? "Revoke your current session? You'll be logged out."
                            : `Revoke ${s.username}'s session?`,
                        )
                      ) {
                        await revokeSession.mutateAsync({ id: s.id });
                        invalidate();
                      }
                    }}
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
