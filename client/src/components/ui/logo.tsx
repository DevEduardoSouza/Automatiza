import { useTheme } from 'next-themes';

interface LogoProps {
  className?: string;
  alt?: string;
}

/**
 * Logo component with automatic dark/light theme support.
 * Uses next-themes to detect current theme and display appropriate logo variant.
 */
export function Logo({
  className = 'h-10 w-auto object-contain',
  alt = 'Automatisa Grupos',
}: LogoProps) {
  const { theme } = useTheme();

  return (
    <img
      src={theme === 'dark' ? '/novalogo_dark.png' : '/novalogo.png'}
      alt={alt}
      className={className}
    />
  );
}
