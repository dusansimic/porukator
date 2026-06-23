import { useEffect, useState } from "react";
import { useQuery, useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { AdminService } from "@/gen/porukator/v1/porukator_pb";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";

export function Settings() {
  const qc = useQueryClient();
  const { data } = useQuery(AdminService.method.getSettings, {});
  const update = useMutation(AdminService.method.updateSettings);

  const [delayMs, setDelayMs] = useState(0);
  const [jitterMs, setJitterMs] = useState(0);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (data?.settings) {
      setDelayMs(data.settings.delayMs);
      setJitterMs(data.settings.jitterMs);
    }
  }, [data]);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    await update.mutateAsync({ settings: { delayMs, jitterMs } });
    setSaved(true);
    qc.invalidateQueries();
    setTimeout(() => setSaved(false), 1500);
  }

  return (
    <div className="space-y-6 max-w-lg">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Settings</h1>
        <p className="text-muted-foreground text-sm">Pacing applied between each SMS a device sends.</p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Send pacing</CardTitle>
          <CardDescription>
            A device waits <strong>delay</strong> plus a random value up to <strong>jitter</strong> between sends.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSave} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="delay">Delay (ms)</Label>
              <Input id="delay" type="number" min={0} value={delayMs} onChange={(e) => setDelayMs(Number(e.target.value))} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="jitter">Jitter (ms)</Label>
              <Input id="jitter" type="number" min={0} value={jitterMs} onChange={(e) => setJitterMs(Number(e.target.value))} />
            </div>
            <div className="flex items-center gap-3">
              <Button type="submit" disabled={update.isPending}>Save</Button>
              {saved && <span className="text-sm text-emerald-500">Saved.</span>}
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
