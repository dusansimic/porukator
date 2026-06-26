import { useState } from "react";
import { useQuery, useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { QRCodeSVG } from "qrcode.react";
import { Plus, Trash2, Pencil, Copy } from "lucide-react";
import { AdminService, type Client } from "@/gen/porukator/v1/porukator_pb";
import { useAuthStore, isAdmin } from "@/stores/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogTrigger,
} from "@/components/ui/dialog";

// Connection payload encoded in the QR a device scans during setup.
function qrPayload(host: string, token: string, name: string) {
  return JSON.stringify({ host, token, name });
}

export function Clients() {
  const qc = useQueryClient();
  const showOwner = isAdmin(useAuthStore((s) => s.user));
  const { data } = useQuery(AdminService.method.listClients, {}, { refetchInterval: 3000 });
  const create = useMutation(AdminService.method.createClient);
  const rename = useMutation(AdminService.method.renameClient);
  const revoke = useMutation(AdminService.method.revokeClient);

  const [newName, setNewName] = useState("");
  const [created, setCreated] = useState<{ name: string; token: string; host: string } | null>(null);
  const [addOpen, setAddOpen] = useState(false);

  const invalidate = () => qc.invalidateQueries();

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    const res = await create.mutateAsync({ name: newName });
    setCreated({ name: res.client?.name ?? newName, token: res.accessToken, host: res.host });
    setNewName("");
    invalidate();
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Devices</h1>
          <p className="text-muted-foreground text-sm">Android phones that send SMS for the gateway.</p>
        </div>
        <Dialog open={addOpen} onOpenChange={(o) => { setAddOpen(o); if (!o) setCreated(null); }}>
          <DialogTrigger asChild>
            <Button><Plus className="h-4 w-4" /> Add device</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add device</DialogTitle>
              <DialogDescription>
                {created
                  ? "Scan this QR in the Porukator app, or copy the token. It is shown only once."
                  : "Give the device a name. You'll get a one-time access token and QR code."}
              </DialogDescription>
            </DialogHeader>
            {created ? (
              <div className="space-y-4">
                <div className="flex justify-center rounded-lg bg-white p-4">
                  <QRCodeSVG value={qrPayload(created.host, created.token, created.name)} size={220} />
                </div>
                <div className="space-y-1">
                  <Label>Access token</Label>
                  <div className="flex gap-2">
                    <Input readOnly value={created.token} className="font-mono text-xs" />
                    <Button variant="outline" size="icon" onClick={() => navigator.clipboard.writeText(created.token)}>
                      <Copy className="h-4 w-4" />
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">Host: {created.host}</p>
                </div>
              </div>
            ) : (
              <form onSubmit={onCreate} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="name">Device name</Label>
                  <Input id="name" value={newName} autoFocus onChange={(e) => setNewName(e.target.value)} />
                </div>
                <Button type="submit" disabled={!newName || create.isPending}>Create</Button>
              </form>
            )}
          </DialogContent>
        </Dialog>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Status</TableHead>
            <TableHead>Name</TableHead>
            {showOwner && <TableHead>Owner</TableHead>}
            <TableHead>Last seen</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(data?.clients ?? []).map((c: Client) => (
            <TableRow key={c.id}>
              <TableCell>
                {c.online ? <Badge variant="success">online</Badge> : <Badge variant="secondary">offline</Badge>}
              </TableCell>
              <TableCell className="font-medium">{c.name}</TableCell>
              {showOwner && (
                <TableCell className="text-muted-foreground">{c.ownerUsername || "—"}</TableCell>
              )}
              <TableCell className="text-muted-foreground">
                {c.lastSeenAt ? new Date(Number(c.lastSeenAt.seconds) * 1000).toLocaleString() : "—"}
              </TableCell>
              <TableCell className="text-right space-x-1">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={async () => {
                    const name = prompt("New name", c.name);
                    if (name && name !== c.name) {
                      await rename.mutateAsync({ id: c.id, name });
                      invalidate();
                    }
                  }}
                >
                  <Pencil className="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={async () => {
                    if (confirm(`Revoke ${c.name}? Its token stops working.`)) {
                      await revoke.mutateAsync({ id: c.id });
                      invalidate();
                    }
                  }}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {data && data.clients.length === 0 && (
            <TableRow>
              <TableCell colSpan={showOwner ? 5 : 4} className="text-center text-muted-foreground py-8">
                No devices yet.
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
