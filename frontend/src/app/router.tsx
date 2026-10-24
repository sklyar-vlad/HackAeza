import type { ComponentType } from "react";
import { createBrowserRouter, Outlet } from "react-router";
import { NotFoundPage, ServerErrorPage } from "@/pages/error/ErrorPage";
import { ErrorDialog } from "@/widgets/error-dialog/ErrorDialog";
import { AppLayout } from "@/widgets/layout/AppLayout";
import { Protected } from "./Protected";

// Each screen is its own chunk; only the shell stays in the entry bundle.
const page =
  <K extends string>(load: () => Promise<Record<K, ComponentType>>, name: K) =>
  async () => ({ Component: (await load())[name] });

function Root() {
  return (
    <>
      <Outlet />
      <ErrorDialog />
    </>
  );
}

export const router = createBrowserRouter([
  {
    element: <Root />,
    errorElement: <ServerErrorPage />,
    children: [
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
      { path: "*", element: <NotFoundPage /> },
    ],
  },
]);
