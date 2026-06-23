import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useMutation } from "@connectrpc/connect-query";
import { AdminService } from "@/gen/porukator/v1/porukator_pb";
import { useAuthStore } from "@/stores/auth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";

export function Login() {
  const [password, setLocalPassword] = useState("");
  const [error, setError] = useState("");
  const setPassword = useAuthStore((s) => s.setPassword);
  const navigate = useNavigate();
  const login = useMutation(AdminService.method.login);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const res = await login.mutateAsync({ password });
      if (res.ok) {
        setPassword(password);
        navigate("/clients", { replace: true });
      } else {
        setError("Incorrect password.");
      }
    } catch {
      setError("Could not reach the service.");
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Porukator</CardTitle>
          <CardDescription>Enter the master password to manage the gateway.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="pw">Master password</Label>
              <Input
                id="pw"
                type="password"
                value={password}
                autoFocus
                onChange={(e) => setLocalPassword(e.target.value)}
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" className="w-full" disabled={login.isPending || !password}>
              {login.isPending ? "Signing in…" : "Sign in"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
