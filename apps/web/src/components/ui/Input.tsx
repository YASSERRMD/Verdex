import { forwardRef, type InputHTMLAttributes } from 'react';
import clsx from 'clsx';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, helperText, id, className, disabled, ...rest }, ref) => {
    const inputId = id ?? (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined);
    const errorId = error && inputId ? `${inputId}-error` : undefined;
    const helperId = helperText && inputId ? `${inputId}-helper` : undefined;

    return (
      <div className="flex flex-col gap-1">
        {label && (
          <label htmlFor={inputId} className="text-sm font-medium text-neutral-700">
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          disabled={disabled}
          aria-invalid={!!error}
          aria-describedby={clsx(errorId, helperId) || undefined}
          className={clsx(
            'block w-full rounded-lg border px-3 py-2.5 text-sm shadow-sm',
            'placeholder:text-neutral-400',
            'focus:outline-none focus:ring-2',
            'disabled:cursor-not-allowed disabled:bg-neutral-50 disabled:text-neutral-400',
            error
              ? 'border-red-400 focus:border-red-500 focus:ring-red-500/30'
              : 'border-neutral-300 focus:border-primary-DEFAULT focus:ring-primary-DEFAULT/30',
            className,
          )}
          {...rest}
        />
        {error && (
          <p id={errorId} role="alert" className="text-xs text-red-600">
            {error}
          </p>
        )}
        {helperText && !error && (
          <p id={helperId} className="text-xs text-neutral-500">
            {helperText}
          </p>
        )}
      </div>
    );
  },
);

Input.displayName = 'Input';
