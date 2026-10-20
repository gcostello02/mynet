/** MyNet mark: three member nodes around a central coordinator point. */
export function LogoMark({ className = "h-5 w-5" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className={className} aria-hidden="true">
      <path
        d="M12 5.2 5.6 17.6M12 5.2l6.4 12.4M5.6 17.6h12.8"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        opacity="0.55"
      />
      <circle cx="12" cy="5.2" r="2.4" fill="currentColor" />
      <circle cx="5.6" cy="17.6" r="2.4" fill="currentColor" />
      <circle cx="18.4" cy="17.6" r="2.4" fill="currentColor" />
      <circle cx="12" cy="13.2" r="1.5" fill="currentColor" opacity="0.9" />
    </svg>
  );
}

export function Logo({ size = "md" }: { size?: "md" | "lg" }) {
  const box = size === "lg" ? "h-10 w-10" : "h-7 w-7";
  const icon = size === "lg" ? "h-6 w-6" : "h-[18px] w-[18px]";
  return (
    <span className="inline-flex items-center gap-2.5">
      <span
        className={`flex ${box} items-center justify-center rounded-lg border border-line-strong bg-raised text-accent`}
      >
        <LogoMark className={icon} />
      </span>
      <span className={`font-semibold tracking-tight ${size === "lg" ? "text-xl" : "text-[15px]"}`}>
        MyNet
      </span>
    </span>
  );
}
