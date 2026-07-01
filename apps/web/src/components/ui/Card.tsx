import type { HTMLAttributes, ReactNode } from 'react';
import clsx from 'clsx';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  header?: ReactNode;
  footer?: ReactNode;
  padding?: 'none' | 'sm' | 'md' | 'lg';
}

const paddingMap = {
  none: '',
  sm: 'px-4 py-3',
  md: 'px-6 py-5',
  lg: 'px-8 py-6',
};

export function Card({
  header,
  footer,
  children,
  padding = 'md',
  className,
  ...rest
}: CardProps) {
  return (
    <div
      className={clsx(
        'rounded-xl border border-neutral-200 bg-white shadow-sm',
        'dark:border-neutral-700 dark:bg-neutral-800',
        className,
      )}
      {...rest}
    >
      {header && (
        <div className="border-b border-neutral-200 px-6 py-4 dark:border-neutral-700">
          {header}
        </div>
      )}
      <div className={clsx(paddingMap[padding])}>{children}</div>
      {footer && (
        <div className="border-t border-neutral-200 px-6 py-4 dark:border-neutral-700">
          {footer}
        </div>
      )}
    </div>
  );
}
