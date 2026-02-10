import {
  Toast,
  ToastClose,
  ToastDescription,
  ToastProvider,
  ToastTitle,
  ToastViewport,
} from '@/components/ui/toast';
import { useToast } from '@/hooks/use-toast';

export function Toaster() {
  const { toasts } = useToast();

  return (
    <ToastProvider duration={5000}>
      {toasts.map(({ id, title, description, action, ...props }) => (
        <Toast key={id} {...props}>
          <div className="grid gap-1">
            {title && <ToastTitle>{title}</ToastTitle>}
            {description && <ToastDescription>{description}</ToastDescription>}
          </div>
          {action}
          <ToastClose />
          <div className="absolute bottom-0 left-0 right-0 h-[5px] bg-current opacity-15 pointer-events-none" />
          <div className="absolute bottom-0 left-0 h-[5px] w-full origin-left bg-current opacity-50 pointer-events-none animate-toast-progress" />
        </Toast>
      ))}
      <ToastViewport />
    </ToastProvider>
  );
}
