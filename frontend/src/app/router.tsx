import type { ComponentType } from "react";
import { createBrowserRouter, Navigate } from "react-router";
import { AppLayout } from "@/widgets/layout/AppLayout";
import { Protected } from "./Protected";

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
    element: (
      <Protected>
        <AppLayout />
      </Protected>
    ),
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
      {
        path: "model",
        lazy: page(() => import("@/pages/model/ModelPage"), "ModelPage"),
      },
      {
        path: "source",
        lazy: page(() => import("@/pages/source/SourcePage"), "SourcePage"),
      },
    ],
  },
  { path: "*", element: <Navigate to="/" replace /> },
]);
