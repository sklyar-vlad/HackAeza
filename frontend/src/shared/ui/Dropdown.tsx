import { AnimatePresence, motion } from "motion/react";
import { useEffect, useId, useRef, useState } from "react";
import { Icon } from "./Icon";
import styles from "./Dropdown.module.css";

export type Option<T extends string> = {
  value: T;
  label: string;
  count?: number;
  dotClass?: string;
};

type Props<T extends string> = {
  label: string;
  value: T;
  options: Option<T>[];
  onChange: (value: T) => void;
};

export function Dropdown<T extends string>({
  label,
  value,
  options,
  onChange,
}: Props<T>) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const listId = useId();
  const current = options.find((o) => o.value === value) ?? options[0];

  useEffect(() => {
    if (!open) return;
    const close = (e: Event) => {
      if (
        e instanceof KeyboardEvent
          ? e.key === "Escape"
          : !root.current?.contains(e.target as Node)
      )
        setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", close);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", close);
    };
  }, [open]);

  return (
    <div ref={root} className={styles.root}>
      <button
        type="button"
        className={styles.trigger}
        data-active={open || value !== options[0].value}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        onClick={() => setOpen(!open)}
      >
        <span className="muted">{label}</span>
        <span className={styles.value}>
          {current.dotClass && <i className={`dot ${current.dotClass}`} />}
          {current.label}
        </span>
        <Icon name="chevron" size={16} className={styles.chevron} />
      </button>
      <AnimatePresence>
        {open && (
          <motion.div
            id={listId}
            role="listbox"
            aria-label={label}
            className={styles.list}
            initial={{ opacity: 0, y: -6, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -6, scale: 0.98 }}
            transition={{ duration: 0.18, ease: [0.4, 0, 0.2, 1] }}
          >
            {options.map((o) => (
              <button
                key={o.value}
                type="button"
                role="option"
                aria-selected={o.value === value}
                className={styles.option}
                onClick={() => {
                  onChange(o.value);
                  setOpen(false);
                }}
              >
                {o.dotClass && <i className={`dot ${o.dotClass}`} />}
                <span className={styles.optionLabel}>{o.label}</span>
                {o.count !== undefined && (
                  <span className="muted num">{o.count}</span>
                )}
                <Icon name="check" size={16} className={styles.check} />
              </button>
            ))}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
