import type { SVGProps } from "react";

// Outline icons from the my.aeza.net UI kit (24px grid, 1.5 stroke).
const paths = {
  home: "M5.78 10.22V18c0 1.1.9 2 2 2H12M5.78 10.22l5.51-5.51a1 1 0 0 1 1.42 0l4.79 4.79M5.78 10.22 4 12M18.22 10.22V18c0 1.1-.9 2-2 2H12M18.22 10.22 20 12M18.22 10.22l-.72-.72M17.5 9.5V6M12 20v-5",
  users:
    "M3 19v-1a4 4 0 0 1 4-4h4a4 4 0 0 1 4 4v1M15 11a3 3 0 1 0 0-6M21 19v-1a4 4 0 0 0-4-4h-.5M12 8a3 3 0 1 1-6 0 3 3 0 0 1 6 0z",
  trend:
    "M3 17l5.29-5.29a1 1 0 0 1 1.42 0l2.58 2.58a1 1 0 0 0 1.42 0L21 7M21 12V7h-5",
  doc: "M19 8.63V18a3 3 0 0 1-3 3H8a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3h5.5L19 8.63zM13.5 3v4.63c0 .55.45 1 1 1H19M9 13h6M9 17h6",
  bookmark:
    "M7 7.2c0-1.12 0-1.68.22-2.11a2 2 0 0 1 .87-.87C8.52 4 9.08 4 10.2 4h3.6c1.12 0 1.68 0 2.11.22.38.19.68.5.87.87.22.43.22.99.22 2.11V20l-2.87-2.55c-.76-.67-1.13-1-1.56-1.13a2 2 0 0 0-1.14 0c-.43.13-.8.46-1.56 1.13L7 20V7.2z",
  external: "M7 7h10v10M17 7 7 17",
  sliders:
    "M4 6h9M17 4v4M17 6h3M4 18h6M14 16v4M14 18h6M20 12h-9M7 14v-4M7 12H4",
  logout:
    "M1.5 12H15M15 12l-3-3M15 12l-3 3M7.5 18v1.5c0 .83.67 1.5 1.5 1.5h9c.83 0 1.5-.67 1.5-1.5v-15c0-.83-.67-1.5-1.5-1.5H9c-.83 0-1.5.67-1.5 1.5V6",
  sun: "M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10zM12 1v2M12 21v2M4.2 4.2l1.4 1.4M18.4 18.4l1.4 1.4M1 12h2M21 12h2M4.2 19.8l1.4-1.4M18.4 5.6l1.4-1.4",
  moon: "M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z",
  search:
    "M10.36 19.22a8.86 8.86 0 1 0 0-17.72 8.86 8.86 0 0 0 0 17.72zM17 17l4 4",
  chevron: "M6 9l6 6 6-6",
  back: "M15 18l-6-6 6-6",
  next: "M9 18l6-6-6-6",
  check: "M20 6 9 17l-5-5",
  eye: "M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z",
  eyeOff:
    "M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19M14.12 14.12a3 3 0 1 1-4.24-4.24M1 1l22 22",
  upload: "M12 15V3M7 8l5-5 5 5M4 15v4a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-4",
  info: "M12 11.2v4M19.2 12a7.2 7.2 0 1 1-14.4 0 7.2 7.2 0 0 1 14.4 0zM12 8.4h.01",
  x: "M18 6 6 18M6 6l12 12",
} as const;

export type IconName = keyof typeof paths;

export function Icon({
  name,
  size = 20,
  ...rest
}: { name: IconName; size?: number } & SVGProps<SVGSVGElement>) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.5}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      {...rest}
    >
      <path d={paths[name]} />
    </svg>
  );
}
