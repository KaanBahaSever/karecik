// Custom product badges.
// The icon catalogue mirrors backend/internal/utils/appearance.go (utils.BadgeIcons)
// exactly — same ids, same order.
//
// NOTE: labels stay Turkish — they are shown in the dashboard badge picker.
//
// The lucide components are imported by name on purpose: a typo becomes a build
// error instead of a silently blank badge.

import { createElement } from 'react'
import {
  Star,
  Sparkles,
  Flame,
  Crown,
  Award,
  Heart,
  ThumbsUp,
  TrendingUp,
  ChefHat,
  Utensils,
  Leaf,
  Vegan,
  Wheat,
  Carrot,
  Coffee,
  CupSoda,
  Wine,
  Beer,
  Milk,
  IceCream,
  Cake,
  Cookie,
  Croissant,
  Donut,
  Candy,
  Pizza,
  Sandwich,
  Salad,
  Soup,
  Snowflake,
  Percent,
  Tag,
} from 'lucide-react'

/** Mirrors models.MaxBadges / models.MaxBadgeTextRunes on the backend. */
export const MAX_BADGES = 5
export const MAX_BADGE_TEXT = 24

/* ------------------------------------------------------------------ icons */

export const BADGE_ICONS = [
  { id: 'star', label: 'Yıldız', Icon: Star },
  { id: 'sparkles', label: 'Parıltı', Icon: Sparkles },
  { id: 'flame', label: 'Acı / Ateş', Icon: Flame },
  { id: 'crown', label: 'Taç', Icon: Crown },
  { id: 'award', label: 'Ödül', Icon: Award },
  { id: 'heart', label: 'Kalp', Icon: Heart },
  { id: 'thumbs-up', label: 'Beğeni', Icon: ThumbsUp },
  { id: 'trending-up', label: 'Yükselen', Icon: TrendingUp },
  { id: 'chef-hat', label: 'Şef', Icon: ChefHat },
  { id: 'utensils', label: 'Çatal Bıçak', Icon: Utensils },
  { id: 'leaf', label: 'Yaprak', Icon: Leaf },
  { id: 'vegan', label: 'Vegan', Icon: Vegan },
  { id: 'wheat', label: 'Buğday', Icon: Wheat },
  { id: 'carrot', label: 'Havuç', Icon: Carrot },
  { id: 'coffee', label: 'Kahve', Icon: Coffee },
  { id: 'cup-soda', label: 'Soğuk İçecek', Icon: CupSoda },
  { id: 'wine', label: 'Şarap', Icon: Wine },
  { id: 'beer', label: 'Bira', Icon: Beer },
  { id: 'milk', label: 'Süt', Icon: Milk },
  { id: 'ice-cream', label: 'Dondurma', Icon: IceCream },
  { id: 'cake', label: 'Pasta', Icon: Cake },
  { id: 'cookie', label: 'Kurabiye', Icon: Cookie },
  { id: 'croissant', label: 'Kruvasan', Icon: Croissant },
  { id: 'donut', label: 'Donut', Icon: Donut },
  { id: 'candy', label: 'Şeker', Icon: Candy },
  { id: 'pizza', label: 'Pizza', Icon: Pizza },
  { id: 'sandwich', label: 'Sandviç', Icon: Sandwich },
  { id: 'salad', label: 'Salata', Icon: Salad },
  { id: 'soup', label: 'Çorba', Icon: Soup },
  { id: 'snowflake', label: 'Kar Tanesi', Icon: Snowflake },
  { id: 'percent', label: 'İndirim', Icon: Percent },
  { id: 'tag', label: 'Etiket', Icon: Tag },
]

/** Looks up a badge icon by its kebab-case id. Unknown ids give null. */
export function findBadgeIcon(id) {
  if (!id) return null
  return BADGE_ICONS.find((icon) => icon.id === id) || null
}

/**
 * Renders one badge icon. An empty or unknown id renders nothing, so a badge
 * saved with an icon we no longer ship simply loses its glyph.
 *
 * Written with createElement instead of JSX: this is a .js module and Vite only
 * parses JSX in .jsx files.
 */
export function BadgeIcon({ id, className }) {
  const entry = findBadgeIcon(id)
  if (!entry) return null
  return createElement(entry.Icon, { className, 'aria-hidden': true })
}

/* ----------------------------------------------------------------- colors */

/** The badge a fresh row in the dashboard builder starts from. */
export const DEFAULT_BADGE = {
  text: '',
  icon: 'star',
  bg_color: '#1d4ed8',
  text_color: '#ffffff',
}

// Ready-made pairings; every one clears WCAG AA for small text.
export const BADGE_COLOR_PRESETS = [
  { bg: '#1d4ed8', fg: '#ffffff', label: 'Mavi' },
  { bg: '#7c3aed', fg: '#ffffff', label: 'Mor' },
  { bg: '#be123c', fg: '#ffffff', label: 'Bordo' },
  { bg: '#c2410c', fg: '#ffffff', label: 'Turuncu' },
  { bg: '#0f766e', fg: '#ffffff', label: 'Petrol' },
  { bg: '#111827', fg: '#ffffff', label: 'Antrasit' },
  { bg: '#fef3c7', fg: '#92400e', label: 'Krem' },
  { bg: '#dcfce7', fg: '#166534', label: 'Açık Yeşil' },
]
