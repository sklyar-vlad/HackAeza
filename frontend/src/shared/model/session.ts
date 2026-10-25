import { create } from "zustand";
import { persist } from "zustand/middleware";

type Session = {
  token: string | null;
  username: string | null;
  signIn: (token: string, username: string) => void;
  signOut: () => void;
};

export const useSession = create<Session>()(
  persist(
    (set) => ({
      token: null,
      username: null,
      signIn: (token, username) => set({ token, username }),
      signOut: () => set({ token: null, username: null }),
    }),
    { name: "aeza-risk-session" },
  ),
);

type Prefs = {
  theme: "light" | "dark";
  cookiesAccepted: boolean;
  toggleTheme: () => void;
  acceptCookies: () => void;
};

export const usePrefs = create<Prefs>()(
  persist(
    (set) => ({
      theme: window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light",
      cookiesAccepted: false,
      toggleTheme: () =>
        set((s) => ({ theme: s.theme === "dark" ? "light" : "dark" })),
      acceptCookies: () => set({ cookiesAccepted: true }),
    }),
    { name: "aeza-risk-prefs" },
  ),
);
