import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Copy, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
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
import { AdminService, type ApiToken } from "@/gen/porukator/v1/porukator_pb";
import { isAdmin, useAuthStore } from "@/stores/auth";

export function ApiTokens() {
  const qc = useQueryClient();
  const showOwner = isAdmin(useAuthStore((s) => s.user));
  const { data } = useQuery(AdminService.method.listApiTokens, {});
  const create = useMutation(AdminService.method.createApiToken);
  const revoke = useMutation(AdminService.method.revokeApiToken);

  const [newName, setNewName] = useState("");
  const [secret, setSecret] = useState<string | null>(null);
  const [open, setOpen] = useState(false);

  const invalidate = () => qc.invalidateQueries();

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    const res = await create.mutateAsync({ name: newName });
    setSecret(res.secret);
    setNewName("");
    invalidate();
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">API Tokens</h1>
          <p className="text-muted-foreground text-sm">
            Credentials for upstream services that submit messages.
          </p>
        </div>
        <Dialog
          open={open}
          onOpenChange={(o) => {
            setOpen(o);
            if (!o) setSecret(null);
          }}
        >
          <DialogTrigger asChild>
            <Button>
              <Plus className="h-4 w-4" /> New token
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>New API token</DialogTitle>
              <DialogDescription>
                {secret
                  ? "Copy this token now — it is shown only once."
                  : "Name the consuming service."}
              </DialogDescription>
            </DialogHeader>
            {secret ? (
              <div className="flex gap-2">
                <Input readOnly value={secret} className="font-mono text-xs" />
                <Button
                  variant="outline"
                  size="icon"
                  onClick={() => navigator.clipboard.writeText(secret)}
                >
                  <Copy className="h-4 w-4" />
                </Button>
              </div>
            ) : (
              <form onSubmit={onCreate} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="tname">Name</Label>
                  <Input
                    id="tname"
                    value={newName}
                    autoFocus
                    onChange={(e) => setNewName(e.target.value)}
                  />
                </div>
                <Button type="submit" disabled={!newName || create.isPending}>
                  Create
                </Button>
              </form>
            )}
          </DialogContent>
        </Dialog>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            {showOwner && <TableHead>Owner</TableHead>}
            <TableHead>Created</TableHead>
            <TableHead>Last used</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(data?.tokens ?? []).map((t: ApiToken) => (
            <TableRow key={t.id}>
              <TableCell className="font-medium">{t.name}</TableCell>
              {showOwner && (
                <TableCell className="text-muted-foreground">{t.ownerUsername || "—"}</TableCell>
              )}
              <TableCell className="text-muted-foreground">
                {t.createdAt ? new Date(Number(t.createdAt.seconds) * 1000).toLocaleString() : "—"}
              </TableCell>
              <TableCell className="text-muted-foreground">
                {t.lastUsedAt
                  ? new Date(Number(t.lastUsedAt.seconds) * 1000).toLocaleString()
                  : "never"}
              </TableCell>
              <TableCell className="text-right">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={async () => {
                    if (confirm(`Revoke ${t.name}?`)) {
                      await revoke.mutateAsync({ id: t.id });
                      invalidate();
                    }
                  }}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {data && data.tokens.length === 0 && (
            <TableRow>
              <TableCell
                colSpan={showOwner ? 5 : 4}
                className="text-center text-muted-foreground py-8"
              >
                No tokens yet.
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
