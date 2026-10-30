/**
 * Maps the navigation icon names to their lucide glyphs (wi-14 S7).
 *
 * `nav-config.ts` declares the icons as names so it can stay a plain `.ts`
 * module that the server components import without dragging React into the
 * module graph. This component is the single place that knows the mapping.
 */

import {
  CalendarDays,
  ClipboardList,
  Coins,
  FileSignature,
  LayoutDashboard,
  Package,
  Receipt,
  ScrollText,
  UserCog,
  Users,
  Wallet,
  type LucideIcon,
} from "lucide-react";

import type { NavIconName } from "@/lib/nav-config";

const ICONS: Record<NavIconName, LucideIcon> = {
  "layout-dashboard": LayoutDashboard,
  "calendar-days": CalendarDays,
  users: Users,
  "clipboard-list": ClipboardList,
  "file-signature": FileSignature,
  receipt: Receipt,
  wallet: Wallet,
  coins: Coins,
  package: Package,
  "user-cog": UserCog,
  "scroll-text": ScrollText,
};

export interface NavIconProps {
  name: NavIconName;
  className?: string;
}

export function NavIcon({ name, className }: NavIconProps) {
  const Icon = ICONS[name];
  return <Icon className={className} aria-hidden="true" />;
}
