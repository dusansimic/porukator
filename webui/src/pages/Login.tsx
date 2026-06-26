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
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const setSession = useAuthStore((s) => s.setSession);
  const navigate = useNavigate();
  const login = useMutation(AdminService.method.login);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const res = await login.mutateAsync({ username, password });
      if (res.token && res.user) {
        setSession(res.token, {
          id: res.user.id,
          username: res.user.username,
          role: res.user.role,
        });
        navigate("/clients", { replace: true });
      } else {
        setError("Login failed.");
      }
    } catch {
      setError("Incorrect username or password.");
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Porukator</CardTitle>
          <CardDescription>Sign in to manage the gateway.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="username">Username</Label>
              <Input id="username" value={username} autoFocus onChange={(e) => setUsername(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="pw">Password</Label>
              <Input id="pw" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" className="w-full" disabled={login.isPending || !username || !password}>
              {login.isPending ? "Signing in…" : "Sign in"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
