import type { ComponentProps, Ref } from 'react';

import { cn } from '@/lib/utils';

type CardProps = ComponentProps<'div'> & {
  ref?: Ref<HTMLDivElement>;
};

function Card({ className, ref, ...props }: CardProps) {
  return (
    <div
      ref={ref}
      className={cn('rounded-lg border bg-card text-card-foreground shadow-sm', className)}
      {...props}
    />
  );
}

type CardHeaderProps = ComponentProps<'div'> & {
  ref?: Ref<HTMLDivElement>;
};

function CardHeader({ className, ref, ...props }: CardHeaderProps) {
  return <div ref={ref} className={cn('flex flex-col space-y-1.5 p-6', className)} {...props} />;
}

type CardTitleProps = ComponentProps<'h3'> & {
  ref?: Ref<HTMLHeadingElement>;
};

function CardTitle({ className, ref, ...props }: CardTitleProps) {
  return (
    <h3
      ref={ref}
      className={cn('text-2xl font-semibold leading-none tracking-tight', className)}
      {...props}
    />
  );
}

type CardDescriptionProps = ComponentProps<'p'> & {
  ref?: Ref<HTMLParagraphElement>;
};

function CardDescription({ className, ref, ...props }: CardDescriptionProps) {
  return <p ref={ref} className={cn('text-sm text-muted-foreground', className)} {...props} />;
}

type CardContentProps = ComponentProps<'div'> & {
  ref?: Ref<HTMLDivElement>;
};

function CardContent({ className, ref, ...props }: CardContentProps) {
  return <div ref={ref} className={cn('p-6 pt-0', className)} {...props} />;
}

type CardFooterProps = ComponentProps<'div'> & {
  ref?: Ref<HTMLDivElement>;
};

function CardFooter({ className, ref, ...props }: CardFooterProps) {
  return <div ref={ref} className={cn('flex items-center p-6 pt-0', className)} {...props} />;
}

export { Card, CardHeader, CardFooter, CardTitle, CardDescription, CardContent };
