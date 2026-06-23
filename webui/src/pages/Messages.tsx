import { useState } from "react";
import { useQuery } from "@connectrpc/connect-query";
import { AdminService, MessageStatus, type Message } from "@/gen/porukator/v1/porukator_pb";
import { Badge } from "@/components/ui/badge";
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table";

const STATUS_LABEL: Record<MessageStatus, string> = {
  [MessageStatus.UNSPECIFIED]: "—",
  [MessageStatus.PENDING]: "pending",
  [MessageStatus.DISPATCHED]: "dispatched",
  [MessageStatus.SENT]: "sent",
  [MessageStatus.FAILED]: "failed",
};

function statusBadge(s: MessageStatus) {
  if (s === MessageStatus.SENT) return <Badge variant="success">sent</Badge>;
  if (s === MessageStatus.FAILED) return <Badge variant="destructive">failed</Badge>;
  if (s === MessageStatus.DISPATCHED) return <Badge>dispatched</Badge>;
  return <Badge variant="secondary">{STATUS_LABEL[s]}</Badge>;
}

function ts(t?: { seconds: bigint }) {
  return t ? new Date(Number(t.seconds) * 1000).toLocaleString() : "—";
}

export function Messages() {
  const [status, setStatus] = useState<MessageStatus>(MessageStatus.UNSPECIFIED);
  const { data } = useQuery(
    AdminService.method.listMessages,
    { limit: 200, status, clientId: "" },
    { refetchInterval: 3000 },
  );

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Messages</h1>
          <p className="text-muted-foreground text-sm">SMS tracked by the gateway, newest first.</p>
        </div>
        <select
          className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
          value={status}
          onChange={(e) => setStatus(Number(e.target.value) as MessageStatus)}
        >
          <option value={MessageStatus.UNSPECIFIED}>All statuses</option>
          <option value={MessageStatus.PENDING}>Pending</option>
          <option value={MessageStatus.DISPATCHED}>Dispatched</option>
          <option value={MessageStatus.SENT}>Sent</option>
          <option value={MessageStatus.FAILED}>Failed</option>
        </select>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Status</TableHead>
            <TableHead>Phone</TableHead>
            <TableHead>Content</TableHead>
            <TableHead>Received</TableHead>
            <TableHead>Sent</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {(data?.messages ?? []).map((m: Message) => (
            <TableRow key={m.id}>
              <TableCell>{statusBadge(m.status)}</TableCell>
              <TableCell className="font-mono text-xs">{m.phoneNumber}</TableCell>
              <TableCell className="max-w-md truncate" title={m.content}>
                {m.content}
                {m.error && <span className="text-destructive"> · {m.error}</span>}
              </TableCell>
              <TableCell className="text-muted-foreground">{ts(m.receivedAt)}</TableCell>
              <TableCell className="text-muted-foreground">{ts(m.sentAt)}</TableCell>
            </TableRow>
          ))}
          {data && data.messages.length === 0 && (
            <TableRow>
              <TableCell colSpan={5} className="text-center text-muted-foreground py-8">
                No messages.
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}
