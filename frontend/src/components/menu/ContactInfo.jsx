import { useEffect, useId, useRef, useState } from 'react'
import {
  Check,
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  Facebook,
  Globe,
  Instagram,
  KeyRound,
  Linkedin,
  MapPin,
  MessageCircle,
  Phone,
  Twitter,
  Wifi,
  Youtube,
} from 'lucide-react'

import { copyToClipboard } from '../../lib/clipboard'
import { t } from '../../locales/index.js'

/**
 * The venue's contact details on the customer menu: Wi-Fi, Instagram, the phone
 * number and the owner's own links.
 *
 * Which of them exist is decided by buildContactItems in lib/contact.js; the
 * components here only lay those items out, in the shapes `contact_display`
 * asks for:
 *
 *   ContactBar            'inline' - one centred, wrapping row of chips on the
 *                         home view; a tap opens that item's details below it
 *   ContactList           'list'   - the same items with their details open
 *   ContactList compact   the footer of the product screens, in every mode
 *                         but 'hidden'
 *
 * On the home view the chips sit under the header: one wrapping row holds every
 * detail, where a card per detail would take a card's height each.
 *
 * An empty item list renders nothing - no row, no card, no margin - so a menu
 * with no contact details looks exactly like one that never had the feature.
 *
 * Every component is declared at module level. The dashboard preview re-renders
 * MenuContent on each keystroke in the settings form; a component declared
 * inside that render would be a new type every time, and the open chip would
 * close and a revealed password hide itself again on every keystroke.
 *
 * Colours come from the --menu-* custom properties, like the rest of the menu,
 * so all six themes work without a line of theme-specific code. The one
 * exception is the open chip's text, which takes the readable-on-accent colour
 * MenuContent already computes for its own accent-filled chips.
 */

/** How long the "Kopyalandı" confirmation stays on a copy button. */
const COPY_FEEDBACK_MS = 1600

/** The most dots a masked password shows, so a long password does not leak its length. */
const MAX_MASK_DOTS = 12

/**
 * The class of a value that is never truncated and must never run past the edge
 * of its line: `break-anywhere` in index.css. See the note above WifiDetails.
 */
const WRAP_VALUE = 'break-anywhere'

/** lucide component per linkIcon() id. WhatsApp gets the speech bubble. */
const LINK_ICONS = {
  whatsapp: MessageCircle,
  maps: MapPin,
  instagram: Instagram,
  facebook: Facebook,
  youtube: Youtube,
  twitter: Twitter,
  linkedin: Linkedin,
  globe: Globe,
}

const PANEL_STYLE = {
  backgroundColor: 'var(--menu-surface)',
  border: '1px solid var(--menu-border)',
  borderRadius: 'var(--menu-radius)',
  boxShadow: 'var(--menu-shadow)',
}

const PILL_STYLE = { border: '1px solid var(--menu-border)', color: 'var(--menu-muted)' }

/** The records of an `items` prop; anything else - a stray null, a non-array - is left out. */
function itemList(items) {
  return Array.isArray(items)
    ? items.filter((item) => item !== null && typeof item === 'object')
    : []
}

/**
 * One React key per item, unique within the list whatever the items carry.
 *
 * buildContactItems already gives every item a unique key, even when stored
 * link ids repeat or are blank. This keeps that promise for any list handed to
 * these components: a repeated or missing key would make React mix up two rows,
 * and ContactBar tell two chips apart by nothing.
 */
function uniqueKeys(items) {
  const used = new Set()
  return items.map((item, index) => {
    const own = typeof item.key === 'string' && item.key !== '' ? item.key : `item-${index}`
    let key = own
    for (let copy = 2; used.has(key); copy += 1) key = `${own}~${copy}`
    used.add(key)
    return key
  })
}

function itemIcon(item) {
  if (item.type === 'wifi') return Wifi
  if (item.type === 'instagram') return Instagram
  if (item.type === 'phone') return Phone
  return Object.prototype.hasOwnProperty.call(LINK_ICONS, item.icon) ? LINK_ICONS[item.icon] : Globe
}

/** The short word on a chip. A custom link is called what the owner called it. */
function chipLabel(item, language) {
  if (item.type === 'wifi') return t('wifiChip', language)
  if (item.type === 'instagram') return t('instagram', language)
  if (item.type === 'phone') return t('phone', language)
  return item.label
}

/**
 * The pill every action beside a value is drawn as. In the compact footer it
 * matches the footer's small type; in the panel and the list it is a notch
 * larger, because there it is what a thumb has to hit.
 */
function pillClassName(compact) {
  return compact
    ? 'inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium'
    : 'inline-flex shrink-0 items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium'
}

/* ---------------------------------------------------------------- actions */

/** "Kopyala", which confirms or reports a failure for a moment, then resets. */
function CopyButton({ value, label, language, compact }) {
  const [state, setState] = useState('idle') // idle | copied | failed
  const timer = useRef(null)

  useEffect(() => () => clearTimeout(timer.current), [])

  async function handleCopy() {
    const ok = await copyToClipboard(value)
    setState(ok ? 'copied' : 'failed')
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setState('idle'), COPY_FEEDBACK_MS)
  }

  const text =
    state === 'copied'
      ? t('copied', language)
      : state === 'failed'
        ? t('copyFailed', language)
        : t('copy', language)

  return (
    <button
      type="button"
      onClick={handleCopy}
      aria-label={`${label}: ${text}`}
      className={pillClassName(compact)}
      style={PILL_STYLE}
    >
      {state === 'copied' ? (
        <Check className="h-3 w-3" aria-hidden="true" />
      ) : (
        <Copy className="h-3 w-3" aria-hidden="true" />
      )}
      {text}
    </button>
  )
}

/**
 * A link beside a value.
 *
 * A web address opens in a new tab with no opener and no referrer: the site an
 * owner linked to gets neither a handle on the menu's window nor the menu's
 * address. A `tel:` link is handed to the dialler rather than opened as a page,
 * so it is left as a plain link.
 */
function ActionLink({ href, external = false, icon: Icon, children, compact }) {
  return (
    <a
      href={href}
      {...(external ? { target: '_blank', rel: 'noopener noreferrer' } : null)}
      className={pillClassName(compact)}
      style={PILL_STYLE}
    >
      {Icon ? <Icon className="h-3 w-3" aria-hidden="true" /> : null}
      {children}
      {external ? <ExternalLink className="h-3 w-3" aria-hidden="true" /> : null}
    </a>
  )
}

/**
 * Shows or hides the Wi-Fi password. The label stays the same in both states and
 * `aria-pressed` carries the state, which is how a toggle button is announced.
 */
function RevealButton({ revealed, onToggle, language, compact }) {
  const label = t('showPassword', language)

  return (
    <button
      type="button"
      onClick={onToggle}
      aria-label={label}
      aria-pressed={revealed}
      title={label}
      className={`inline-flex shrink-0 items-center justify-center rounded-full ${
        compact ? 'h-5 w-5' : 'h-7 w-7'
      }`}
      style={{ color: 'var(--menu-muted)' }}
    >
      {revealed ? (
        <EyeOff className="h-3.5 w-3.5" aria-hidden="true" />
      ) : (
        <Eye className="h-3.5 w-3.5" aria-hidden="true" />
      )}
    </button>
  )
}

/* ----------------------------------------------------------------- layout */

/** The item's icon in the accent colour, with its value lines beside it. */
function DetailFrame({ icon: Icon, children }) {
  return (
    <div className="flex items-start gap-3">
      <Icon
        className="mt-0.5 h-4 w-4 shrink-0"
        style={{ color: 'var(--menu-primary)' }}
        aria-hidden="true"
      />
      <div className="min-w-0 flex-1 space-y-2.5">{children}</div>
    </div>
  )
}

/**
 * One value in the panel or the list: an optional small caption above it, an
 * optional muted note below it, and its actions after it.
 *
 * The actions sit beside the value while both fit on the line and wrap onto a
 * line of their own below it when they do not, so a narrow phone never cuts a
 * value short to make room for its buttons. A value too wide even for a line of
 * its own is handled by `wrap`:
 *
 *   wrap=false  the value and the note truncate, and `title`, where the caller
 *               passes one, carries the full value (a network name, a label)
 *   wrap=true   the value continues on the next line and is never cut (a phone
 *               number, a password, text with no link to fall back on)
 */
function ValueLine({ caption, value, title, note, actions, wrap = false }) {
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1.5">
      <div className="min-w-0 flex-auto">
        {caption ? (
          <p className="mb-1 text-[11px] leading-none" style={{ color: 'var(--menu-muted)' }}>
            {caption}
          </p>
        ) : null}
        <p
          className={`text-sm font-medium leading-snug ${wrap ? WRAP_VALUE : 'truncate'}`}
          style={{ color: 'var(--menu-text)' }}
          title={title}
        >
          {value}
        </p>
        {note ? (
          <p className="mt-0.5 truncate text-[11px] leading-snug" style={{ color: 'var(--menu-muted)' }}>
            {note}
          </p>
        ) : null}
      </div>
      {actions ? (
        <div className="ms-auto flex shrink-0 flex-wrap items-center gap-1.5">{actions}</div>
      ) : null}
    </div>
  )
}

/**
 * One value in the compact footer list, on a single centred line that wraps
 * when it must.
 *
 * Nothing in the footer is truncated, so the value and the note carry
 * WRAP_VALUE. The caption is plain words and wraps at its spaces.
 *
 * `valueBelowCaption` starts the value on a line of its own under the caption,
 * for the revealed Wi-Fi password. Read on the caption's line it could start
 * right after the caption and continue on the next line, which is where a
 * browser that falls back to word-break: break-all splits it.
 */
function CompactLine({ icon: Icon, caption, value, note, actions, valueBelowCaption = false }) {
  return (
    <div className="flex flex-wrap items-center justify-center gap-1.5">
      <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <span className="min-w-0">
        {caption ? (valueBelowCaption ? `${caption}:` : `${caption}: `) : ''}
        <span
          className={valueBelowCaption ? `block font-medium ${WRAP_VALUE}` : `font-medium ${WRAP_VALUE}`}
          style={{ color: 'var(--menu-text)' }}
        >
          {value}
        </span>
        {note ? <span className={WRAP_VALUE}> · {note}</span> : null}
      </span>
      {actions}
    </div>
  )
}

/* ---------------------------------------------------------------- details */

/*
  Values that are addresses or numbers sit in <bdi dir="ltr">. In the Arabic menu
  the surrounding text runs right to left, and without the isolation a phone
  number's leading "+" or a hostname's dots are reordered around it. Free text
  the owner typed - a network name, a link label - gets a bare <bdi>, which
  picks its direction from its own first letters.

  A value that is never truncated - the Wi-Fi password, the phone number, an
  Instagram value with no user name, and every value in the compact footer -
  carries WRAP_VALUE, which ValueLine (with `wrap`) and CompactLine apply. One
  longer than its line then breaks inside itself instead of running past the
  edge of a narrow screen, and only where the line offers no other break:
  "+90 (532) 000 00 00" still wraps at its spaces. index.css holds the rule and
  its fallback for a browser without overflow-wrap: anywhere.
*/

/**
 * Wi-Fi: the network name with a copy button, and the password MASKED with a
 * reveal toggle and a copy button. Copying never needs the password on screen,
 * and it is never printed in the clear until the customer asks for it - a table
 * of strangers can read a phone screen too.
 *
 * The reveal state lives here, so it resets whenever these details unmount:
 * closing the chip, or opening another one, masks the password again.
 */
function WifiDetails({ item, language, compact }) {
  const [revealed, setRevealed] = useState(false)

  const { ssid, password } = item
  const masked = '•'.repeat(Math.min(password.length, MAX_MASK_DOTS))
  const passwordValue = <bdi dir="ltr">{revealed ? password : masked}</bdi>

  const ssidActions = (
    <CopyButton
      value={ssid}
      label={t('wifiName', language)}
      language={language}
      compact={compact}
    />
  )
  const passwordActions = (
    <>
      <RevealButton
        revealed={revealed}
        onToggle={() => setRevealed((visible) => !visible)}
        language={language}
        compact={compact}
      />
      <CopyButton
        value={password}
        label={t('wifiPassword', language)}
        language={language}
        compact={compact}
      />
    </>
  )

  if (compact) {
    return (
      <div className="flex flex-col items-center gap-2">
        {ssid ? (
          <CompactLine
            icon={Wifi}
            caption={t('wifiName', language)}
            value={<bdi>{ssid}</bdi>}
            actions={ssidActions}
          />
        ) : null}
        {password ? (
          <CompactLine
            icon={KeyRound}
            caption={t('wifiPassword', language)}
            value={passwordValue}
            actions={passwordActions}
            valueBelowCaption={revealed}
          />
        ) : null}
      </div>
    )
  }

  return (
    <DetailFrame icon={Wifi}>
      {ssid ? (
        <ValueLine
          caption={t('wifiName', language)}
          value={<bdi>{ssid}</bdi>}
          title={ssid}
          actions={ssidActions}
        />
      ) : null}
      {password ? (
        <ValueLine
          caption={t('wifiPassword', language)}
          value={passwordValue}
          actions={passwordActions}
          wrap
        />
      ) : null}
    </DetailFrame>
  )
}

/**
 * The details of one item: in the open panel of the chip row, in a row of the
 * list, or - `compact` - on a line of the footer.
 */
function ContactDetails({ item, language, compact = false }) {
  if (item.type === 'wifi') {
    return <WifiDetails item={item} language={language} compact={compact} />
  }

  const icon = itemIcon(item)

  if (item.type === 'instagram') {
    /* A field that holds no user name has no profile to open, so its text is
       shown as the owner typed it, with no button - the item is never simply
       dropped. */
    const text =
      typeof item.text === 'string' && item.text !== ''
        ? item.text
        : item.handle
          ? `@${item.handle}`
          : ''
    const linked = Boolean(item.url)
    const value = linked ? <bdi dir="ltr">{text}</bdi> : <bdi>{text}</bdi>
    const actions = linked ? (
      <ActionLink href={item.url} external compact={compact}>
        {t('openInstagram', language)}
      </ActionLink>
    ) : null

    return compact ? (
      <CompactLine icon={icon} value={value} actions={actions} />
    ) : (
      <DetailFrame icon={icon}>
        <ValueLine
          caption={t('instagram', language)}
          value={value}
          title={text}
          actions={actions}
          wrap={!linked}
        />
      </DetailFrame>
    )
  }

  if (item.type === 'phone') {
    const value = <bdi dir="ltr">{item.phone}</bdi>
    const actions = (
      <>
        <ActionLink href={item.href} icon={Phone} compact={compact}>
          {t('call', language)}
        </ActionLink>
        <CopyButton
          value={item.phone}
          label={t('phone', language)}
          language={language}
          compact={compact}
        />
      </>
    )

    return compact ? (
      <CompactLine icon={icon} value={value} actions={actions} />
    ) : (
      <DetailFrame icon={icon}>
        <ValueLine caption={t('phone', language)} value={value} actions={actions} wrap />
      </DetailFrame>
    )
  }

  // A custom link: its label, where it goes, and the way there.
  const value = <bdi>{item.label}</bdi>
  const note = <bdi dir="ltr">{item.hostname}</bdi>
  const actions = (
    <ActionLink href={item.url} external compact={compact}>
      {t('openLink', language)}
    </ActionLink>
  )

  return compact ? (
    <CompactLine icon={icon} value={value} note={note} actions={actions} />
  ) : (
    <DetailFrame icon={icon}>
      <ValueLine value={value} title={item.label} note={note} actions={actions} />
    </DetailFrame>
  )
}

/* ------------------------------------------------------------- components */

/**
 * `contact_display: 'inline'` - a centred, wrapping row of chips with one detail
 * panel below it.
 *
 * At most one chip is open at a time, and tapping the open chip closes it. The
 * selection is kept by item key. When the open item leaves the list - a link the
 * owner just deleted in the dashboard preview - the selection is cleared, so the
 * panel stays closed if an item with the same key comes back later.
 *
 * The panel element stays in the page while closed, `hidden`, so every chip's
 * `aria-controls` always points at something that exists.
 *
 * @param {object[]} items        - buildContactItems(business)
 * @param {string}   language     - Active language code
 * @param {string}   onAccentText - Text colour that reads on the accent colour
 * @param {string}   className    - Outer spacing; applied only when there are items
 */
export function ContactBar({ items, language = 'tr', onAccentText = '#ffffff', className = '' }) {
  const [openKey, setOpenKey] = useState(null)
  const panelId = useId()

  const list = itemList(items)
  const keys = uniqueKeys(list)
  const openIndex = openKey === null ? -1 : keys.indexOf(openKey)
  const openItem = openIndex === -1 ? null : list[openIndex]

  // Before the early return below: an empty list must clear the selection too.
  const openItemGone = openKey !== null && openIndex === -1
  useEffect(() => {
    if (openItemGone) setOpenKey(null)
  }, [openItemGone])

  if (list.length === 0) return null

  return (
    <div className={className || undefined}>
      <div
        role="group"
        aria-label={t('contact', language)}
        className="flex flex-wrap justify-center gap-2"
      >
        {list.map((item, index) => {
          const key = keys[index]
          const isOpen = index === openIndex
          const Icon = itemIcon(item)

          return (
            <button
              key={key}
              type="button"
              onClick={() => setOpenKey(isOpen ? null : key)}
              aria-expanded={isOpen}
              aria-controls={panelId}
              title={item.type === 'link' ? item.label : undefined}
              className="inline-flex min-w-0 max-w-[11rem] items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium"
              style={
                /* The border is there in both states, in the accent colour when
                   open, so opening a chip never changes its size and nudges the
                   row. */
                isOpen
                  ? {
                      backgroundColor: 'var(--menu-primary)',
                      border: '1px solid var(--menu-primary)',
                      color: onAccentText,
                    }
                  : {
                      backgroundColor: 'var(--menu-surface)',
                      border: '1px solid var(--menu-border)',
                      color: 'var(--menu-text)',
                    }
              }
            >
              <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              <span className="truncate">{chipLabel(item, language)}</span>
            </button>
          )
        })}
      </div>

      <div id={panelId} hidden={!openItem} className="mt-2 px-3.5 py-3" style={PANEL_STYLE}>
        {/* Keyed by item, so switching from one chip to another starts the new
            details from scratch - a password revealed under Wi-Fi is masked
            again the next time Wi-Fi opens. */}
        {openItem ? (
          <ContactDetails key={keys[openIndex]} item={openItem} language={language} />
        ) : null}
      </div>
    </div>
  )
}

/**
 * `contact_display: 'list'` - every item with its details always open, one row
 * each on a single card. With `compact` it is the footer's variant instead: no
 * card, one centred line per value, at the footer's small size.
 *
 * @param {object[]} items     - buildContactItems(business)
 * @param {string}   language  - Active language code
 * @param {boolean}  compact   - The product screens' footer variant
 * @param {string}   className - Outer spacing; applied only when there are items
 */
export function ContactList({ items, language = 'tr', compact = false, className = '' }) {
  const list = itemList(items)
  if (list.length === 0) return null

  const keys = uniqueKeys(list)

  if (compact) {
    return (
      <ul
        aria-label={t('contact', language)}
        className={`flex flex-col items-center gap-2 text-xs ${className}`.trim()}
      >
        {list.map((item, index) => (
          <li key={keys[index]} className="max-w-full">
            <ContactDetails item={item} language={language} compact />
          </li>
        ))}
      </ul>
    )
  }

  return (
    <ul aria-label={t('contact', language)} className={className || undefined} style={PANEL_STYLE}>
      {list.map((item, index) => (
        <li
          key={keys[index]}
          className="px-3.5 py-3"
          style={index > 0 ? { borderTop: '1px solid var(--menu-border)' } : undefined}
        >
          <ContactDetails item={item} language={language} />
        </li>
      ))}
    </ul>
  )
}
