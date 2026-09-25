// The two free-text fields of a product and the line between them.
//
//   description  a short introduction or serving note: "Günlük kavrulan
//                çekirdeklerle, sıcak servis edilir."
//   ingredients  the comma-separated list of what is in it: "espresso, süt,
//                kakao" - the customer menu prints it under its own
//                "İçindekiler" heading in the product's detail sheet
//
// Imported menus often carry the same text in both, and the customer menu
// then drops the description as a repeat (productTexts in lib/category.js).
// The product dialog points that out with a gentle warning - it never blocks
// the save - and this is the test it uses.
//
// The comparison is the customer menu's own comparableText, not a copy of it:
// the dialog must warn about exactly the pairs the menu treats as repeats.
//
// Pure functions only - no React, no DOM - so frontend/tests/
// dashboardSettings.test.mjs runs them with plain node.

import { comparableText } from './category.js'

/**
 * Whether a product's description and ingredients say the same thing once
 * case, spacing and trailing punctuation are set aside. Two empty fields do
 * not: leaving both blank is fine, and so is filling in only one.
 */
export function descriptionRepeatsIngredients(description, ingredients) {
  const first = comparableText(description)
  return first !== '' && first === comparableText(ingredients)
}
