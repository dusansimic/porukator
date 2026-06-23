import { createConnectTransport } from "@connectrpc/connect-web";
import type { Interceptor } from "@connectrpc/connect";
import { useAuthStore } from "@/stores/auth";

// Attach the master password as a bearer token on every request. The Login RPC
// is exempt server-side, so an unset password still lets the login call through.
const authInterceptor: Interceptor = (next) => async (req) => {
  const pw = useAuthStore.getState().password;
  if (pw) {
    req.header.set("Authorization", `Bearer ${pw}`);
  }
  return next(req);
};

// baseUrl is the page origin; service paths like /porukator.v1.AdminService/...
// are proxied to the Go server in dev and served same-origin in prod.
export const transport = createConnectTransport({
  baseUrl: window.location.origin,
  useBinaryFormat: false,
  interceptors: [authInterceptor],
});
