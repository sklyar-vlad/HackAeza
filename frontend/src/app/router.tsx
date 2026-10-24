import type { ComponentType } from "react";
import { createBrowserRouter, Navigate } from "react-router";
import { useSession } from "@/shared/model/session";
import { AppLayout } from "@/widgets/layout/AppLayout";

function Protected() {
  const token = useSession((s) => s.token);
  return token ? <AppLayout /> : <Navigate to="/login" replace />;
}

// Each screen is its own chunk; only the shell stays in the entry bundle.
const page =
  <K extends string>(load: () => Promise<Record<K, ComponentType>>, name: K) =>
  async () => ({ Component: (await load())[name] });

export const router = createBrowserRouter([
  {
    path: "/login",
    lazy: page(() => import("@/pages/login/LoginPage"), "LoginPage"),
  },
  {
    element: <Protected />,
    children: [
      {
        index: true,
        lazy: page(
          () => import("@/pages/overview/OverviewPage"),
          "OverviewPage",
        ),
      },
      {
        path: "customers",
        lazy: page(
          () => import("@/pages/customers/CustomersPage"),
          "CustomersPage",
        ),
      },
      {
        path: "customers/:id",
        lazy: page(
          () => import("@/pages/customer/CustomerPage"),
          "CustomerPage",
        ),
      },
    ],
  },
  { path: "*", element: <Navigate to="/" replace /> },
]);
