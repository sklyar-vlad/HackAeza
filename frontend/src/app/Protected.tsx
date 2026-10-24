import type { ReactNode } from "react";
import { Navigate } from "react-router";
import { useGetCurrentUser } from "@/shared/api/generated/api";
import { useSession } from "@/shared/model/session";

export function Protected({ children }: { children: ReactNode }) {
  const { token, signOut } = useSession();
  const user = useGetCurrentUser({
    query: {
      queryKey: ["session", token],
      enabled: Boolean(token),
      retry: false,
      staleTime: 0,
      refetchInterval: 60_000,
    },
  });

  if (!token) return <Navigate to="/login" replace />;
  if (user.isPending) return <p role="status">Проверяем сессию…</p>;
  if (user.isError)
    return (
      <div role="alert">
        <p>Не удалось проверить сессию. Проверьте соединение с сервером.</p>
        <button className="btn" onClick={() => void user.refetch()}>
          Повторить
        </button>
        <button className="btn" onClick={signOut}>
          К форме входа
        </button>
      </div>
    );
  return children;
}
