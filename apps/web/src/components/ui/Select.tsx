import { forwardRef, type SelectHTMLAttributes } from 'react';
import clsx from 'clsx';

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  error?: string;
  helperText?: string;
  options: SelectOption[];
  placeholder?: string;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  (
    { label, error, helperText, options, placeholder, id, className, disabled, ...rest },
    ref,
  ) => {
    const selectId = id ?? (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined);
    const errorId = error && selectId ? `${selectId}-error` : undefined;
    const helperId = helperText && selectId ? `${selectId}-helper` : undefined;

    return (
      <div className="flex flex-col gap-1">
        {label && (
          <label htmlFor={selectId} className="text-sm font-medium text-neutral-700">
            {label}
          </label>
        )}
        <select
          ref={ref}
          id={selectId}
          disabled={disabled}
          aria-invalid={!!error}
          aria-describedby={clsx(errorId, helperId) || undefined}
          className={clsx(
            'block w-full rounded-lg border px-3 py-2.5 text-sm shadow-sm',
            'bg-white',
            'focus:outline-none focus:ring-2',
            'disabled:cursor-not-allowed disabled:bg-neutral-50 disabled:text-neutral-400',
            error
              ? 'border-red-400 focus:border-red-500 focus:ring-red-500/30'
              : 'border-neutral-300 focus:border-primary-DEFAULT focus:ring-primary-DEFAULT/30',
            className,
          )}
          {...rest}
        >
          {placeholder && (
            <option value="" disabled>
              {placeholder}
            </option>
          )}
          {options.map(({ value, label: optLabel, disabled: optDisabled }) => (
            <option key={value} value={value} disabled={optDisabled}>
              {optLabel}
            </option>
          ))}
        </select>
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

Select.displayName = 'Select';
