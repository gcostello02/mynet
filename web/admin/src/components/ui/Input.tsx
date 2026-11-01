import { ChevronDown, Search, X } from "lucide-react";
import {
  forwardRef,
  useId,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
} from "react";

const field =
  "w-full rounded-md border border-line-strong bg-canvas px-2.5 text-[13px] text-fg placeholder:text-fg-subtle transition-colors hover:border-fg-subtle/60 focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/25 disabled:opacity-50";

export function Field({
  label,
  hint,
  error,
  htmlFor,
  children,
}: {
  label?: string;
  hint?: ReactNode;
  error?: string | null;
  htmlFor: string;
  children: ReactNode;
}) {
  return (
    <div>
      {label && (
        <label htmlFor={htmlFor} className="mb-1.5 block text-xs font-medium text-fg-muted">
          {label}
        </label>
      )}
      {children}
      {error ? (
        <p className="mt-1.5 text-xs text-bad" role="alert">
          {error}
        </p>
      ) : (
        hint && <p className="mt-1.5 text-xs leading-relaxed text-fg-subtle">{hint}</p>
      )}
    </div>
  );
}

export const Input = forwardRef<
  HTMLInputElement,
  InputHTMLAttributes<HTMLInputElement> & { label?: string; hint?: ReactNode; error?: string | null }
>(function Input({ label, hint, error, className = "", id, ...props }, ref) {
  const auto = useId();
  const inputId = id ?? auto;
  return (
    <Field label={label} hint={hint} error={error} htmlFor={inputId}>
      <input
        ref={ref}
        id={inputId}
        aria-invalid={error ? true : undefined}
        className={`${field} h-8 ${className}`}
        {...props}
      />
    </Field>
  );
});

export function Select({
  label,
  hint,
  className = "",
  children,
  id,
  ...props
}: SelectHTMLAttributes<HTMLSelectElement> & { label?: string; hint?: ReactNode }) {
  const auto = useId();
  const selectId = id ?? auto;
  return (
    <Field label={label} hint={hint} htmlFor={selectId}>
      <div className="relative">
        <select id={selectId} className={`${field} h-8 appearance-none pr-8 ${className}`} {...props}>
          {children}
        </select>
        <ChevronDown
          className="pointer-events-none absolute right-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-fg-subtle"
          aria-hidden
        />
      </div>
    </Field>
  );
}

export const SearchInput = forwardRef<
  HTMLInputElement,
  {
    value: string;
    onChange: (v: string) => void;
    placeholder?: string;
    label: string;
    className?: string;
  }
>(function SearchInput({ value, onChange, placeholder, label, className = "" }, ref) {
  return (
    <div className={`relative ${className}`}>
      <Search
        className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-fg-subtle"
        aria-hidden
      />
      <input
        ref={ref}
        type="text"
        role="searchbox"
        aria-label={label}
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={`${field} h-8 pl-8 pr-14`}
      />
      {value ? (
        <button
          type="button"
          aria-label="Clear search"
          onClick={() => onChange("")}
          className="absolute right-1.5 top-1/2 flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded text-fg-subtle hover:bg-hover hover:text-fg"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      ) : (
        <kbd className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 rounded border border-line-strong px-1.5 font-mono text-[10px] leading-4 text-fg-subtle">
          /
        </kbd>
      )}
    </div>
  );
});
