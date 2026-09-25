// Per-language names of product option groups and items, for the product
// dialog (components/dashboard/ProductModal.jsx).
//
// The backend stores an option as { name, translations: { en: { name } }, ... }
// (models.OptionTranslations): `name` is the name in the menu's default
// language and the fallback of every other one, and `translations` carries the
// rest. The dialog edits a flat { [language]: string } map per group and per
// item instead and turns it back into that shape on save.

import { parsePrice } from './format.js'

/** Refusals of a row that has a name, but not in the default language. */
export const OPTION_NAME_MESSAGES = {
  groupMissingPrimary: 'Varsayılan dilde grup adı zorunludur.',
  itemMissingPrimary: 'Varsayılan dilde seçenek adı zorunludur.',
}

function text(value) {
  return typeof value === 'string' ? value : ''
}

/**
 * The editable names of one stored option group or item: a string for every
 * menu language, '' where there is none.
 *
 * The primary language reads its own translation before `name`. The two are
 * the same text whenever the backend saved them under the current default
 * language; they differ only after the menu's default language changed, and
 * then the translation is the text written in that language while `name` still
 * holds the old default's.
 *
 * @param {object}   entry           - stored group or item
 * @param {string[]} languages       - the menu's languages
 * @param {string}   primaryLanguage - the menu's default language
 * @returns {Record<string, string>}
 */
export function optionNamesOf(entry, languages, primaryLanguage) {
  const translations =
    entry?.translations && typeof entry.translations === 'object' ? entry.translations : {}
  const names = {}
  languages.forEach((code) => {
    names[code] = text(translations[code]?.name)
  })
  names[primaryLanguage] = text(translations[primaryLanguage]?.name) || text(entry?.name)
  return names
}

/** Whether a name was typed in any language. */
export function hasAnyOptionName(names) {
  return Object.values(names || {}).some((value) => text(value).trim() !== '')
}

/**
 * The `name` and `translations` fields of one group or item in the payload:
 * the primary language becomes `name`, every other named language a
 * translation. Blank languages are left out, and so is an empty map.
 *
 * @returns {{ name: string, translations?: Record<string, { name: string }> }}
 */
export function optionNamePayload(names, languages, primaryLanguage) {
  const fields = { name: text(names?.[primaryLanguage]).trim() }
  const translations = {}
  languages.forEach((code) => {
    if (code === primaryLanguage) return
    const name = text(names?.[code]).trim()
    if (name) translations[code] = { name }
  })
  if (Object.keys(translations).length > 0) fields.translations = translations
  return fields
}

/**
 * Builds the `options` payload from the dialog's rows.
 *
 * A row with no name in any language is an unfinished row and is dropped, as
 * is a group left with no named item — a half-finished row must never block
 * the save. A row named in another language but not in the primary one is
 * refused instead: dropping it would silently throw away what the owner typed,
 * and the backend requires the default-language name.
 *
 * @param {Array}    groups          - rows: { uid, names, type, required, items: [{ uid, names, price }] }
 * @param {string[]} languages
 * @param {string}   primaryLanguage
 * @returns {{ options: Array, problem: null | { uid: string, message: string } }}
 */
export function buildOptionsPayload(groups, languages, primaryLanguage) {
  const options = []

  for (const group of groups || []) {
    if (!hasAnyOptionName(group.names)) continue
    const groupFields = optionNamePayload(group.names, languages, primaryLanguage)
    if (!groupFields.name) {
      return {
        options: [],
        problem: { uid: group.uid, message: OPTION_NAME_MESSAGES.groupMissingPrimary },
      }
    }

    const items = []
    for (const item of group.items || []) {
      if (!hasAnyOptionName(item.names)) continue
      const itemFields = optionNamePayload(item.names, languages, primaryLanguage)
      if (!itemFields.name) {
        return {
          options: [],
          problem: { uid: item.uid, message: OPTION_NAME_MESSAGES.itemMissingPrimary },
        }
      }
      items.push({ ...itemFields, price: parsePrice(item.price) })
    }
    if (items.length === 0) continue

    options.push({
      ...groupFields,
      type: group.type === 'multiple' ? 'multiple' : 'single',
      required: Boolean(group.required),
      items,
    })
  }

  return { options, problem: null }
}
