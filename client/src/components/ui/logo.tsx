interface LogoProps {
  className?: string;
  alt?: string;
}

/**
 * Logo component displaying the light theme variant.
 */
export function Logo({
  className = 'h-10 w-auto object-contain',
  alt = 'Automatisa Grupos',
}: LogoProps) {
  return (
    <img
      src="/novalogo.png"
      alt={alt}
      className={className}
    />
  );
}
