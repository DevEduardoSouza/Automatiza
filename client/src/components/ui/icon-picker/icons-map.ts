import {
  Armchair,
  // Kids & Toys
  Baby,
  Bath,
  Bed,
  Bike,
  Blocks,
  // Books & Education
  Book,
  BookOpen,
  Bookmark,
  Camera,
  // Automotive
  Car,
  Cat,
  Cpu,
  CreditCard,
  Crown,
  Dog,
  Droplets,
  // Sports
  Dumbbell,
  Fish,
  Flower2,
  Footprints,
  Fuel,
  Gamepad2,
  Gem,
  Gift,
  Glasses,
  GraduationCap,
  Headphones,
  Heart,
  // Home
  Home,
  Lamp,
  Laptop,
  Leaf,
  type LucideIcon,
  Medal,
  // Electronics
  Monitor,
  Mountain,
  Package,
  // Pets
  PawPrint,
  Pencil,
  Percent,
  Plane,
  Puzzle,
  Refrigerator,
  Scissors,
  Settings,
  Ship,
  // Fashion
  Shirt,
  ShoppingBag,
  ShoppingCart,
  Smartphone,
  Sofa,
  // Beauty
  Sparkles,
  Speaker,
  Star,
  Store,
  Tablet,
  // General / E-commerce
  Tag,
  Timer,
  Trophy,
  Tv,
  UtensilsCrossed,
  Wallet,
  Watch,
  Wrench,
  Zap,
} from 'lucide-react';

export const ICON_MAP: Record<string, LucideIcon> = {
  // Electronics
  monitor: Monitor,
  smartphone: Smartphone,
  headphones: Headphones,
  laptop: Laptop,
  tablet: Tablet,
  cpu: Cpu,
  tv: Tv,
  camera: Camera,
  gamepad2: Gamepad2,
  speaker: Speaker,
  watch: Watch,
  // Fashion
  shirt: Shirt,
  'shopping-bag': ShoppingBag,
  glasses: Glasses,
  footprints: Footprints,
  crown: Crown,
  gem: Gem,
  // Home
  home: Home,
  sofa: Sofa,
  bed: Bed,
  lamp: Lamp,
  refrigerator: Refrigerator,
  'utensils-crossed': UtensilsCrossed,
  bath: Bath,
  armchair: Armchair,
  // Beauty
  sparkles: Sparkles,
  heart: Heart,
  leaf: Leaf,
  droplets: Droplets,
  scissors: Scissors,
  flower2: Flower2,
  // Sports
  dumbbell: Dumbbell,
  medal: Medal,
  trophy: Trophy,
  bike: Bike,
  timer: Timer,
  mountain: Mountain,
  // Books & Education
  book: Book,
  'book-open': BookOpen,
  bookmark: Bookmark,
  'graduation-cap': GraduationCap,
  pencil: Pencil,
  // Kids & Toys
  baby: Baby,
  gift: Gift,
  puzzle: Puzzle,
  blocks: Blocks,
  // Automotive
  car: Car,
  plane: Plane,
  ship: Ship,
  fuel: Fuel,
  wrench: Wrench,
  // Pets
  'paw-print': PawPrint,
  dog: Dog,
  cat: Cat,
  fish: Fish,
  // General / E-commerce
  tag: Tag,
  star: Star,
  zap: Zap,
  settings: Settings,
  package: Package,
  'shopping-cart': ShoppingCart,
  store: Store,
  percent: Percent,
  'credit-card': CreditCard,
  wallet: Wallet,
};

export const ICON_CATEGORIES: Record<string, string[]> = {
  electronics: [
    'monitor',
    'smartphone',
    'headphones',
    'laptop',
    'tablet',
    'cpu',
    'tv',
    'camera',
    'gamepad2',
    'speaker',
    'watch',
  ],
  fashion: ['shirt', 'shopping-bag', 'glasses', 'footprints', 'crown', 'gem'],
  home: ['home', 'sofa', 'bed', 'lamp', 'refrigerator', 'utensils-crossed', 'bath', 'armchair'],
  beauty: ['sparkles', 'heart', 'leaf', 'droplets', 'scissors', 'flower2'],
  sports: ['dumbbell', 'medal', 'trophy', 'bike', 'timer', 'mountain'],
  education: ['book', 'book-open', 'bookmark', 'graduation-cap', 'pencil'],
  kids: ['baby', 'gift', 'puzzle', 'blocks'],
  automotive: ['car', 'plane', 'ship', 'fuel', 'wrench'],
  pets: ['paw-print', 'dog', 'cat', 'fish'],
  general: [
    'tag',
    'star',
    'zap',
    'settings',
    'package',
    'shopping-cart',
    'store',
    'percent',
    'credit-card',
    'wallet',
  ],
};

export type IconName = keyof typeof ICON_MAP;

export function getIconComponent(name: string): LucideIcon | null {
  return ICON_MAP[name] || null;
}

export function getAllIconNames(): string[] {
  return Object.keys(ICON_MAP);
}
