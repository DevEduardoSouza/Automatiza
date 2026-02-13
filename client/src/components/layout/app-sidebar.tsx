import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  useSidebar,
} from '@/components/ui/sidebar';
import { useAuth } from '@/contexts';
import { useSettingsConnect } from '@/hooks/use-settings-connect';
import { BookOpen, type LucideIcon, Type } from 'lucide-react';

const DashboardIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M18 11H18.009" />
    <path d="M20 4H20.009" />
    <path d="M11 6H11.009" />
    <path d="M11 11H11.009" />
    <path d="M9 16H9.00898" />
    <path d="M21 21H10C6.70017 21 5.05025 21 4.02513 19.9749C3 18.9497 3 17.2998 3 14V3" />
  </svg>
);

const WhatsAppIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M12.0045 11H12.0135M8.00903 11H8.018" />
    <path d="M12 3C10.5209 3 9.09517 3.0309 7.7562 3.08819C5.3157 3.1926 4.09545 3.24481 3.13007 4.21745C2.16469 5.19009 2.12282 6.37683 2.03909 8.7503C2.01346 9.47679 2 10.2292 2 11C2 11.7708 2.01346 12.5232 2.03909 13.2497C2.12282 15.6232 2.16469 16.8099 3.13007 17.7825C4.09545 18.7552 5.31569 18.8074 7.75619 18.9118C7.83715 18.9153 7.91842 18.9186 8 18.9219V21.2701C8 21.6732 8.32679 22 8.72991 22C8.90419 22 9.07273 21.9376 9.20503 21.8242L11.3845 19.9553C11.9325 19.4855 12.2064 19.2506 12.532 19.1266C12.8576 19.0026 13.2282 18.9955 13.9693 18.9815C14.7498 18.9667 15.5098 18.9432 16.2437 18.9118C18.6843 18.8074 19.9046 18.7552 20.8699 17.7826C21.8353 16.8099 21.8772 15.6232 21.9609 13.2497C21.9865 12.5232 22 11.7708 22 11C22 10.2292 21.9865 9.47679 21.9609 8.7503C21.9372 8.07936 21.9169 7.50325 21.8781 7" />
    <path d="M15 5.33333C15 5.33333 16 5.33333 17 7C17 7 19.1765 2.83333 22 2" />
  </svg>
);

const GroupsIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M15.5 11C15.5 9.067 13.933 7.5 12 7.5C10.067 7.5 8.5 9.067 8.5 11C8.5 12.933 10.067 14.5 12 14.5C13.933 14.5 15.5 12.933 15.5 11Z" />
    <path d="M15.4827 11.3499C15.8047 11.4475 16.1462 11.5 16.5 11.5C18.433 11.5 20 9.933 20 8C20 6.067 18.433 4.5 16.5 4.5C14.6851 4.5 13.1928 5.8814 13.0173 7.65013" />
    <path d="M10.9827 7.65013C10.8072 5.8814 9.31492 4.5 7.5 4.5C5.567 4.5 4 6.067 4 8C4 9.933 5.567 11.5 7.5 11.5C7.85381 11.5 8.19535 11.4475 8.51727 11.3499" />
    <path d="M22 16.5C22 13.7386 19.5376 11.5 16.5 11.5" />
    <path d="M17.5 19.5C17.5 16.7386 15.0376 14.5 12 14.5C8.96243 14.5 6.5 16.7386 6.5 19.5" />
    <path d="M7.5 11.5C4.46243 11.5 2 13.7386 2 16.5" />
  </svg>
);

const SchedulesIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M16 2V6M8 2V6" />
    <path d="M13 4H11C7.22876 4 5.34315 4 4.17157 5.17157C3 6.34315 3 8.22876 3 12V14C3 17.7712 3 19.6569 4.17157 20.8284C5.34315 22 7.22876 22 11 22H13C16.7712 22 18.6569 22 19.8284 20.8284C21 19.6569 21 17.7712 21 14V12C21 8.22876 21 6.34315 19.8284 5.17157C18.6569 4 16.7712 4 13 4Z" />
    <path d="M3 10H21" />
    <path d="M11.9955 14H12.0045M11.9955 18H12.0045M15.991 14H16M8 14H8.00897M8 18H8.00897" />
  </svg>
);

const RedirectIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    className={className}
    {...props}
  >
    <path d="M10 13.229C10.1416 13.4609 10.3097 13.6804 10.5042 13.8828C11.7117 15.1395 13.5522 15.336 14.9576 14.4722C15.218 14.3121 15.4634 14.1157 15.6872 13.8828L18.9266 10.5114C20.3578 9.02184 20.3578 6.60676 18.9266 5.11718C17.4953 3.6276 15.1748 3.62761 13.7435 5.11718L13.03 5.85978" />
    <path d="M10.9703 18.14L10.2565 18.8828C8.82526 20.3724 6.50471 20.3724 5.07345 18.8828C3.64218 17.3932 3.64218 14.9782 5.07345 13.4886L8.31287 10.1172C9.74413 8.62761 12.0647 8.6276 13.4959 10.1172C13.6904 10.3195 13.8584 10.539 14 10.7708" />
  </svg>
);

const AIIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M4 12C4 8.22876 4 6.34315 5.17157 5.17157C6.34315 4 8.22876 4 12 4C15.7712 4 17.6569 4 18.8284 5.17157C20 6.34315 20 8.22876 20 12C20 15.7712 20 17.6569 18.8284 18.8284C17.6569 20 15.7712 20 12 20C8.22876 20 6.34315 20 5.17157 18.8284C4 17.6569 4 15.7712 4 12Z" />
    <path d="M7.5 15L9.34189 9.47434C9.43631 9.19107 9.7014 9 10 9C10.2986 9 10.5637 9.19107 10.6581 9.47434L12.5 15M8.5 13H11.5" />
    <path d="M15.5 9V15" />
    <path d="M8 2V4M16 2V4M12 2V4M8 20V22M12 20V22M16 20V22M22 16H20M4 8H2M4 16H2M4 12H2M22 8H20M22 12H20" />
  </svg>
);

const LinkListsIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    className={className}
    {...props}
  >
    <path d="M11 6L21 6" />
    <path d="M11 12L21 12" />
    <path d="M11 18L21 18" />
    <path d="M3 7.39286C3 7.39286 4 8.04466 4.5 9C4.5 9 6 5.25 8 4" />
    <path d="M3 18.3929C3 18.3929 4 19.0447 4.5 20C4.5 20 6 16.25 8 15" />
  </svg>
);

const AffiliateIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M2 12C2 8.46252 2 6.69377 3.0528 5.5129C3.22119 5.32403 3.40678 5.14935 3.60746 4.99087C4.86213 4 6.74142 4 10.5 4H13.5C17.2586 4 19.1379 4 20.3925 4.99087C20.5932 5.14935 20.7788 5.32403 20.9472 5.5129C22 6.69377 22 8.46252 22 12C22 15.5375 22 17.3062 20.9472 18.4871C20.7788 18.676 20.5932 18.8506 20.3925 19.0091C19.1379 20 17.2586 20 13.5 20H10.5C6.74142 20 4.86213 20 3.60746 19.0091C3.40678 18.8506 3.22119 18.676 3.0528 18.4871C2 17.3062 2 15.5375 2 12Z" />
    <path d="M10 16H11.5" strokeMiterlimit="10" />
    <path d="M14.5 16L18 16" strokeMiterlimit="10" />
    <path d="M2 9H22" />
  </svg>
);

const AutomationLogsIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M18 16L19.8398 17.5858C20.6133 18.2525 21 18.5858 21 19C21 19.4142 20.6133 19.7475 19.8398 20.4142L18 22" />
    <path d="M14 16L12.1602 17.5858C11.3867 18.2525 11 18.5858 11 19C11 19.4142 11.3867 19.7475 12.1602 20.4142L14 22" />
    <path d="M20 13.0032L20 7.8199C20 6.12616 20 5.27929 19.732 4.60291C19.3013 3.51555 18.3902 2.65784 17.2352 2.25228C16.5168 2 15.6173 2 13.8182 2C10.6698 2 9.09563 2 7.83836 2.44148C5.81714 3.15122 4.22281 4.6522 3.46894 6.55509C3 7.73875 3 9.22077 3 12.1848L3 14.731C3 17.8013 3 19.3364 3.8477 20.4025C4.09058 20.708 4.37862 20.9792 4.70307 21.2078C5.61506 21.8506 6.85019 21.9757 9 22" />
    <path d="M3 12C3 10.159 4.49238 8.66667 6.33333 8.66667C6.99912 8.66667 7.78404 8.78333 8.43137 8.60988C9.00652 8.45576 9.45576 8.00652 9.60988 7.43136C9.78333 6.78404 9.66667 5.99912 9.66667 5.33333C9.66667 3.49238 11.1591 2 13 2" />
  </svg>
);

const CouponsIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M10.9961 10H11.0111M10.9998 16H11.0148" />
    <path d="M7 13H15" />
    <circle cx="1.5" cy="1.5" r="1.5" transform="matrix(1 0 0 -1 16 8)" />
    <path d="M2.77423 11.1439C1.77108 12.2643 1.7495 13.9546 2.67016 15.1437C4.49711 17.5033 6.49674 19.5029 8.85633 21.3298C10.0454 22.2505 11.7357 22.2289 12.8561 21.2258C15.8979 18.5022 18.6835 15.6559 21.3719 12.5279C21.6377 12.2187 21.8039 11.8397 21.8412 11.4336C22.0062 9.63798 22.3452 4.46467 20.9403 3.05974C19.5353 1.65481 14.362 1.99377 12.5664 2.15876C12.1603 2.19608 11.7813 2.36233 11.472 2.62811C8.34412 5.31646 5.49781 8.10211 2.77423 11.1439Z" />
  </svg>
);

const HistoryIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M19 10.5V9.99995C19 6.22876 18.9999 4.34311 17.8284 3.17154C16.6568 2 14.7712 2 11 2C7.22889 2 5.34326 2.00006 4.17169 3.17159C3.00015 4.34315 3.00013 6.22872 3.0001 9.99988L3.00006 14.5C3.00003 17.7874 3.00002 19.4312 3.90794 20.5375C4.07418 20.7401 4.25992 20.9258 4.46249 21.0921C5.56883 22 7.21255 22 10.5 22" />
    <path d="M7 7H15M7 11H11" />
    <path d="M18 18.5L16.5 17.95V15.5M12 17.5C12 19.9853 14.0147 22 16.5 22C18.9853 22 21 19.9853 21 17.5C21 15.0147 18.9853 13 16.5 13C14.0147 13 12 15.0147 12 17.5Z" />
  </svg>
);

const ProfileIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M2 12C2 8.22876 2 6.34315 3.17157 5.17157C4.34315 4 6.22876 4 10 4H14C17.7712 4 19.6569 4 20.8284 5.17157C22 6.34315 22 8.22876 22 12C22 15.7712 22 17.6569 20.8284 18.8284C19.6569 20 17.7712 20 14 20H10C6.22876 20 4.34315 20 3.17157 18.8284C2 17.6569 2 15.7712 2 12Z" />
    <path d="M9 12.5C7.61929 12.5 6.5 11.3807 6.5 10C6.5 8.61929 7.61929 7.5 9 7.5C10.3807 7.5 11.5 8.61929 11.5 10C11.5 11.3807 10.3807 12.5 9 12.5ZM9 12.5C11.2091 12.5 13 14.2909 13 16.5M9 12.5C6.79086 12.5 5 14.2909 5 16.5" />
    <path d="M15 9H19" />
    <path d="M15 12H19" />
  </svg>
);

const AdminIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M5 20.5H19" />
    <path d="M16.8717 17.5H7.1283C6.10017 17.5 5.58611 17.5 5.19623 17.2234C4.80634 16.9468 4.63649 16.4616 4.29679 15.4912L2.05123 9.07668C1.93172 8.72325 2.02503 8.3336 2.29225 8.07016C2.62854 7.73864 3.15545 7.6872 3.55117 7.94727L4.78349 8.75718C6.02739 9.5747 6.64935 9.98345 7.27815 9.83488C7.90696 9.68631 8.28019 9.04241 9.02665 7.75461L11.2412 3.93412C11.3968 3.66567 11.6864 3.5 12 3.5C12.3136 3.5 12.6032 3.66567 12.7588 3.93412L14.9733 7.75461C15.7198 9.04241 16.093 9.68631 16.7218 9.83488C17.3507 9.98345 17.9726 9.5747 19.2165 8.75718L20.4488 7.94727C20.8445 7.6872 21.3715 7.73864 21.7078 8.07016C21.975 8.3336 22.0683 8.72325 21.9488 9.07668L19.7032 15.4912C19.3635 16.4616 19.1937 16.9468 18.8038 17.2234C18.4139 17.5 17.8998 17.5 16.8717 17.5Z" />
  </svg>
);

const SupportIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M12.0045 11H12.0135M8.00903 11H8.018" />
    <path d="M22 13.2497C21.9163 15.6232 21.8353 16.8099 20.8699 17.7826C19.9046 18.7552 18.6843 18.8074 16.2437 18.9118C15.5098 18.9432 14.7498 18.9667 13.9693 18.9815C13.2282 18.9955 12.8576 19.0026 12.532 19.1266C12.2064 19.2506 11.9325 19.4855 11.3845 19.9553L9.20503 21.8242C9.07273 21.9376 8.90419 22 8.72991 22C8.32679 22 8 21.6732 8 21.2701V18.9219C7.91842 18.9186 7.83715 18.9153 7.75619 18.9118C5.31569 18.8074 4.09545 18.7552 3.13007 17.7825C2.16469 16.8099 2.12282 15.6232 2.03909 13.2497C2.01346 12.5232 2 11.7708 2 11C2 10.2292 2.01346 9.47679 2.03909 8.7503C2.12282 6.37683 2.16469 5.19009 3.13007 4.21745C4.09545 3.24481 5.3157 3.1926 7.7562 3.08819C9.09517 3.0309 10.5209 3 12 3C12.6684 3 13.3258 3.00631 13.9693 3.01851" />
    <path d="M19.491 11H19.5" />
    <path d="M17 4.5C17 3.11929 18.1193 2 19.5 2C20.8807 2 22 3.11929 22 4.5C22 5.53529 21.3707 6.4236 20.4737 6.80327C19.9651 7.01856 19.5 7.44772 19.5 8" />
  </svg>
);


const LogoutIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    <path d="M8.00002 8C8.00002 7.42459 8.00002 7.17765 8.04465 6.92457C8.21993 5.93047 8.89355 5.09255 9.83302 4.70001C10.0723 4.60003 10.3559 4.53526 10.9232 4.40573L13.6508 3.78286C17.0405 3.00882 18.7353 2.6218 19.8677 3.51317C21 4.40454 21 6.1257 21 9.56803L21 14.432C21 17.8743 21 19.5955 19.8676 20.4868C18.7353 21.3782 17.0405 20.9912 13.6508 20.2171L10.9232 19.5943C10.3559 19.4647 10.0723 19.4 9.833 19.3C8.89353 18.9074 8.21991 18.0695 8.04462 17.0754C8 16.8224 8 16.5754 8 16" />
    <path d="M13 9C13 9 16 11.2095 16 12C16 12.7906 13 15 13 15M15.5 12H3" />
  </svg>
);

const ToggleSidebarIcon: LucideIcon = ({ className, ...props }) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="24"
    height="24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.5"
    className={className}
    {...props}
  >
    <path d="M2 12C2 8.3109 2 6.46633 2.81382 5.1588C3.1149 4.67505 3.48891 4.2543 3.91891 3.91557C5.08116 3.00003 6.72077 3.00003 10 3.00003H14C17.2792 3.00003 18.9188 3.00003 20.0811 3.91557C20.5111 4.2543 20.8851 4.67505 21.1862 5.1588C22 6.46633 22 8.3109 22 12C22 15.6892 22 17.5337 21.1862 18.8413C20.8851 19.325 20.5111 19.7458 20.0811 20.0845C18.9188 21 17.2792 21 14 21H10C6.72077 21 5.08116 21 3.91891 20.0845C3.48891 19.7458 3.1149 19.325 2.81382 18.8413C2 17.5337 2 15.6892 2 12Z" />
    <path d="M14.5 3.00003L14.5 21" />
    <path d="M18 7.00006H19M18 10.0001H19" />
  </svg>
);

import { useTranslation } from 'react-i18next';
import { NavLink, useLocation } from 'react-router';

interface MenuItem {
  titleKey: string;
  url: string;
  icon: LucideIcon;
  comingSoon?: boolean;
}

const menuItems: MenuItem[] = [
  { titleKey: 'sidebar.dashboard', url: '/dashboard', icon: DashboardIcon },
  { titleKey: 'sidebar.whatsapp', url: '/whatsapp', icon: WhatsAppIcon },
  { titleKey: 'sidebar.groups', url: '/groups', icon: GroupsIcon },
  { titleKey: 'sidebar.schedules', url: '/jobs', icon: SchedulesIcon },
  { titleKey: 'sidebar.redirect', url: '/redirect', icon: RedirectIcon },
  { titleKey: 'sidebar.offerAutomations', url: '/offer-automations', icon: AIIcon },
  { titleKey: 'sidebar.linkLists', url: '/ml-link-lists', icon: LinkListsIcon, comingSoon: true },
  { titleKey: 'sidebar.dispatchTexts', url: '/dispatch-texts', icon: Type },
  { titleKey: 'sidebar.affiliatePrograms', url: '/affiliate-programs', icon: AffiliateIcon },
  { titleKey: 'sidebar.coupons', url: '/coupons', icon: CouponsIcon },
  { titleKey: 'sidebar.automationLogs', url: '/automation-logs', icon: AutomationLogsIcon },
  { titleKey: 'sidebar.history', url: '/history', icon: HistoryIcon },
  { titleKey: 'sidebar.profile', url: '/profile', icon: ProfileIcon },
  { titleKey: 'sidebar.admin', url: '/admin', icon: AdminIcon },
];

/**
 * Sidebar principal da aplicacao.
 * Exibe navegacao, configuracoes de tema e acoes de usuario.
 * Usa ConnectRPC para buscar configuracoes e role do usuario.
 */
export function AppSidebar() {
  const { t } = useTranslation();
  const { state, isMobile, setOpenMobile, toggleSidebar } = useSidebar();
  const location = useLocation();
  const { signOut, user } = useAuth();
  const currentPath = location.pathname;
  const collapsed = state === 'collapsed';

  // Check if user is admin from auth context role
  const isAdmin = user?.role === 'admin';

  // Fetch app settings via ConnectRPC
  const { getSetting } = useSettingsConnect(['support_whatsapp', 'tutorial_link']);
  const supportWhatsapp = getSetting<string>('support_whatsapp', '5511999999999');
  const tutorialLink = getSetting<string>('tutorial_link', '');

  const handleLinkClick = () => {
    if (isMobile) {
      setOpenMobile(false);
    }
  };

  const visibleMenuItems = menuItems.filter((item) => {
    if (item.url === '/admin') {
      return isAdmin;
    }
    return true;
  });

  const getNavCls = ({ isActive }: { isActive: boolean }) =>
    `sidebar-nav-item ${isActive ? 'sidebar-nav-item-active' : ''}`;

  return (
    <Sidebar collapsible="icon">
      <SidebarContent>
        <div
          className={
            collapsed ? 'px-2 pt-3 mb-4 flex justify-center' : 'px-4 mb-4 flex justify-center'
          }
        >
          {collapsed ? (
            <button
              type="button"
              className="sidebar-nav-item !size-10 justify-center"
              onClick={toggleSidebar}
              aria-label={t('common.toggleSidebar')}
            >
              <ToggleSidebarIcon className="sidebar-nav-icon" />
            </button>
          ) : (
            <button type="button" onClick={toggleSidebar} className="cursor-pointer">
              <img
                src="/novalogo.png"
                alt="Automatisa Grupos"
                className="h-16 w-auto object-contain transition-all duration-300 hover:opacity-80"
              />
            </button>
          )}
        </div>

        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {visibleMenuItems.map((item) => (
                <SidebarMenuItem key={item.titleKey}>
                  <NavLink to={item.url} end className={getNavCls} onClick={handleLinkClick}>
                    <item.icon className="sidebar-nav-icon" />
                    {!collapsed && (
                      <span className="sidebar-nav-text flex items-center gap-2">
                        {t(item.titleKey)}
                        {item.comingSoon && (
                          <span className="text-[9px] font-semibold uppercase tracking-wider bg-primary/10 text-primary px-1.5 py-0.5 rounded">
                            {t('common.comingSoon')}
                          </span>
                        )}
                      </span>
                    )}
                  </NavLink>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <div className={collapsed ? 'mt-auto p-2 space-y-2' : 'mt-auto p-4 space-y-2'}>
          <button
            type="button"
            className={`sidebar-nav-item w-full ${collapsed ? '!size-10 justify-center' : ''}`}
            onClick={() => window.open(`https://wa.me/${supportWhatsapp}`, '_blank')}
          >
            <SupportIcon className="sidebar-nav-icon" />
            {!collapsed && <span className="sidebar-nav-text">{t('sidebar.support')}</span>}
          </button>

          {tutorialLink && (
            <button
              type="button"
              className={`sidebar-nav-item w-full ${collapsed ? '!size-10 justify-center' : ''}`}
              onClick={() => window.open(tutorialLink, '_blank')}
            >
              <BookOpen className="sidebar-nav-icon" />
              {!collapsed && <span className="sidebar-nav-text">{t('sidebar.tutorials')}</span>}
            </button>
          )}

          <button
            type="button"
            className={`sidebar-nav-item w-full ${collapsed ? '!size-10 justify-center' : ''}`}
            onClick={() => signOut()}
          >
            <LogoutIcon className="sidebar-nav-icon" />
            {!collapsed && <span className="sidebar-nav-text">{t('sidebar.logout')}</span>}
          </button>
        </div>
      </SidebarContent>
    </Sidebar>
  );
}
