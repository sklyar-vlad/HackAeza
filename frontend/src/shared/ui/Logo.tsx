import type { CSSProperties } from "react";
import styles from "./Logo.module.css";

export function Logo({ size = 34 }: { size?: number }) {
  return (
    <span
      className={styles.logo}
      style={{ "--s": `${size}px` } as CSSProperties}
      role="img"
      aria-label="aéza"
    >
      {["a", "é", "z", "a"].map((key, i) => (
        <span key={i} aria-hidden="true">
          {key}
        </span>
      ))}
    </span>
  );
}

export function Mark() {
  return (
    <span className={styles.mark} role="img" aria-label="aéza">
      é
    </span>
  );
}
