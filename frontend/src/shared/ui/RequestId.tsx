import { useState } from "react";
import { Icon } from "@/shared/ui/Icon";

export function RequestId({ id }: { id: string }) {
  const [copied, setCopied] = useState(false);
  const copy = () =>
    navigator.clipboard.writeText(id).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });

  return (
    <>
      <span>Код запроса</span>
      <code>{id}</code>
      <button
        type="button"
        className="btn btn-ghost"
        aria-label={copied ? "Скопировано" : "Скопировать код запроса"}
        onClick={copy}
      >
        <Icon name={copied ? "check" : "copy"} size={18} />
      </button>
    </>
  );
}
