import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

// The whole admin UI is gated by a single master password (set at service
// start). We hold it in memory + localStorage and attach it as a bearer token
// to every request via the transport interceptor.
type AuthState = {
  password: string | null;
  setPassword: (pw: string) => void;
  clear: () => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      password: null,
      setPassword: (password) => set({ password }),
      clear: () => set({ password: null }),
    }),
    {
      name: "porukator.auth",
      storage: createJSONStorage(() => localStorage),
      version: 1,
    },
  ),
);
