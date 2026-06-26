import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { Role } from "@/gen/porukator/v1/porukator_pb";

// The web UI authenticates as a user account; Login returns an opaque session
// token we store and attach as a bearer token on every request.
export type SessionUser = {
  id: string;
  username: string;
  role: Role;
};

type AuthState = {
  token: string | null;
  user: SessionUser | null;
  setSession: (token: string, user: SessionUser) => void;
  setUser: (user: SessionUser) => void;
  clear: () => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      setSession: (token, user) => set({ token, user }),
      setUser: (user) => set({ user }),
      clear: () => set({ token: null, user: null }),
    }),
    {
      name: "porukator.auth",
      storage: createJSONStorage(() => localStorage),
      version: 2,
    },
  ),
);

export const isAdmin = (u: SessionUser | null) => u?.role === Role.ADMIN;
